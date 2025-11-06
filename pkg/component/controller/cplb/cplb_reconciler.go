// SPDX-FileCopyrightText: 2024 k0s authors
// SPDX-License-Identifier: Apache-2.0

package cplb

import (
	"context"
	"fmt"
	"slices"
	"sync"
	"time"

	kubeutil "github.com/k0sproject/k0s/pkg/kubernetes"
	"github.com/k0sproject/k0s/pkg/kubernetes/watch"
	"github.com/sirupsen/logrus"
	discoveryv1 "k8s.io/api/discovery/v1"
	"k8s.io/client-go/kubernetes"
)

// CPLBReconciler monitors the endpoints of the "kubernetes" service in the
// "default" namespace. It notifies changes though the updateCh channel provided
// in the constructor.
type CPLBReconciler struct {
	log            *logrus.Entry
	kubeconfigPath string
	addresses      []string
	mu             sync.RWMutex
	updateCh       chan<- struct{}
	stop           func()
}

func NewCPLBReconciler(kubeconfigPath string, updateCh chan<- struct{}) *CPLBReconciler {
	return &CPLBReconciler{
		log:            logrus.WithField("component", "cplb-reconciler"),
		kubeconfigPath: kubeconfigPath,
		updateCh:       updateCh,
	}
}

func (r *CPLBReconciler) Start() error {
	clientset, err := kubeutil.NewClientFromFile(r.kubeconfigPath)
	if err != nil {
		return fmt.Errorf("failed to get clientset: %w", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})

	go func() {
		defer close(done)
		r.watchAPIServers(ctx, clientset)
	}()

	r.stop = func() { cancel(); <-done }

	return nil
}

func (r *CPLBReconciler) Stop() {
	r.log.Debug("Stopping")
	r.stop()
	r.log.Info("Stopped")
}

func (r *CPLBReconciler) watchAPIServers(ctx context.Context, clientSet kubernetes.Interface) {
	var lastObservedVersion string
	_ = watch.EndpointSlices(clientSet.DiscoveryV1().EndpointSlices("default")).
		WithObjectName("kubernetes").
		WithErrorCallback(func(err error) (time.Duration, error) {
			if retryAfter, e := watch.IsRetryable(err); e == nil {
				r.log.WithError(err).Infof(
					"Transient error while watching API server endpoints"+
						", last observed version is %q, starting over in %s ...",
					lastObservedVersion, retryAfter,
				)
				return retryAfter, nil
			}

			retryAfter := 10 * time.Second
			r.log.WithError(err).Errorf(
				"Failed to watch API server endpoints"+
					", last observed version is %q, starting over in %s ...",
				lastObservedVersion, retryAfter,
			)
			return retryAfter, nil
		}).
		Until(ctx, func(endpoints *discoveryv1.EndpointSlice) (bool, error) {
			if lastObservedVersion != endpoints.ResourceVersion {
				lastObservedVersion = endpoints.ResourceVersion
				r.maybeUpdateIPs(endpoints)
			}
			return false, nil
		})
}

// maybeUpdateIPs updates the list of IP addresses if the new list has
// different addresses
func (r *CPLBReconciler) maybeUpdateIPs(es *discoveryv1.EndpointSlice) {
	newAddresses := []string{}
	for _, ep := range es.Endpoints {
		if ep.Conditions.Ready != nil && *ep.Conditions.Ready {
			newAddresses = append(newAddresses, ep.Addresses...)
		}
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	// endpoints are not guaranteed to be sorted by IP address
	slices.Sort(newAddresses)

	if !slices.Equal(r.addresses, newAddresses) {
		r.addresses = newAddresses
		r.log.Infof("Updated the list of IPs: %v", r.addresses)
		select {
		case r.updateCh <- struct{}{}:
		default:
		}
	}
}

// GetIPs gets a thread-safe copy of the current list of IP addresses
func (r *CPLBReconciler) GetIPs() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return slices.Clone(r.addresses)
}
