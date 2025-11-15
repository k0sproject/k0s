// SPDX-FileCopyrightText: 2024 k0s authors
// SPDX-License-Identifier: Apache-2.0

package cplb

import (
	"context"
	"fmt"
	"maps"
	"net"
	"net/http"
	"slices"
	"strconv"
	"sync"
	"time"

	kubeutil "github.com/k0sproject/k0s/pkg/kubernetes"
	"github.com/k0sproject/k0s/pkg/kubernetes/watch"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/sirupsen/logrus"
)

// CPLBReconciler monitors the endpoints of the "kubernetes" service in the
// "default" namespace. It notifies changes though the updateCh channel provided
// in the constructor.
type CPLBReconciler struct {
	apiPort        int
	log            *logrus.Entry
	kubeconfigPath string
	mu             sync.RWMutex
	updateCh       chan<- struct{}
	stop           func()
	healthCheckers map[string]*healthChecker
}

func NewCPLBReconciler(kubeconfigPath string, apiPort int, updateCh chan<- struct{}) *CPLBReconciler {
	return &CPLBReconciler{
		apiPort:        apiPort,
		log:            logrus.WithField("component", "cplb-reconciler"),
		kubeconfigPath: kubeconfigPath,
		updateCh:       updateCh,
		healthCheckers: make(map[string]*healthChecker),
	}
}

func (r *CPLBReconciler) Start() error {
	clientSet, err := kubeutil.NewClientFromFile(r.kubeconfigPath)
	if err != nil {
		return err
	}

	restConfig, err := clientcmd.BuildConfigFromFlags("", r.kubeconfigPath)
	if err != nil {
		return fmt.Errorf("failed to build REST config: %w", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		r.watchAPIServers(ctx, clientSet, restConfig)
	}()

	r.stop = func() { cancel(); <-done }

	return nil
}

func (r *CPLBReconciler) Stop() {
	r.log.Debug("Stopping")
	r.stop()
	r.log.Info("Stopped")
}

func (r *CPLBReconciler) watchAPIServers(ctx context.Context, clientSet kubernetes.Interface, restConfig *rest.Config) {
	var lastObservedVersion string
	_ = watch.Endpoints(clientSet.CoreV1().Endpoints(metav1.NamespaceDefault)).
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
		Until(ctx, func(endpoints *corev1.Endpoints) (bool, error) {
			if lastObservedVersion != endpoints.ResourceVersion {
				lastObservedVersion = endpoints.ResourceVersion
				r.maybeUpdateIPs(ctx, endpoints, restConfig)
			}
			return false, nil
		})
}

// maybeUpdateIPs updates the list of IP addresses if the new list has
// different addresses
func (r *CPLBReconciler) maybeUpdateIPs(ctx context.Context, endpoint *corev1.Endpoints, restConfig *rest.Config) {
	newAddresses := []string{}
	for _, subset := range endpoint.Subsets {
		for _, addr := range subset.Addresses {
			newAddresses = append(newAddresses, addr.IP)
		}
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	for _, addr := range newAddresses {
		if _, ok := r.healthCheckers[addr]; !ok {
			r.healthCheckers[addr] = r.newHealthChecker(ctx, restConfig, addr, nil)
		}
	}

	shouldNotify := false
	for addr := range r.healthCheckers {
		if !slices.Contains(newAddresses, addr) {
			r.healthCheckers[addr].Stop()
			delete(r.healthCheckers, addr)
			shouldNotify = true
		}
	}
	if shouldNotify {
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

	var healthyAddrs []string
	for addr := range r.healthCheckers {
		r.healthCheckers[addr].mu.Lock()
		if r.healthCheckers[addr].healthy {
			healthyAddrs = append(healthyAddrs, addr)
		}
		r.healthCheckers[addr].mu.Unlock()
	}

	if len(healthyAddrs) == 0 && len(r.healthCheckers) > 0 {
		r.log.Warn("No healthy API servers detected by CPLBReconciler, returning every known address")
		return slices.Collect(maps.Keys(r.healthCheckers))
	}
	return healthyAddrs
}

// healthChecker provides health checking functionality for Kubernetes API servers
type healthChecker struct {
	mu            sync.Mutex
	cancel        context.CancelFunc
	rc            rest.Interface
	log           *logrus.Entry
	healthy       bool
	updateCh      chan<- struct{}
	healthCheckFn func(context.Context) restResult
}

// newHealthChecker creates a new healthChecker instance
func (r *CPLBReconciler) newHealthChecker(ctx context.Context, restConfig *rest.Config, addr string, healthCheckFn func(context.Context) restResult) *healthChecker {
	var rc *rest.RESTClient
	var err error

	// In testing we don't provide a restConfig, therefore skip creating the rest client
	if restConfig != nil {
		cfg := rest.CopyConfig(restConfig)
		cfg.GroupVersion = &corev1.SchemeGroupVersion
		cfg.NegotiatedSerializer = scheme.Codecs.WithoutConversion()
		cfg.Host = net.JoinHostPort(addr, strconv.Itoa(r.apiPort))

		// Configure transport to establish new connections on every request
		cfg.WrapTransport = func(rt http.RoundTripper) http.RoundTripper {
			// If the original transport is an *http.Transport, modify it to disable connection reuse
			if httpTransport, ok := rt.(*http.Transport); ok {
				// Clone the transport to avoid modifying the original
				transport := httpTransport.Clone()
				transport.DisableKeepAlives = true // Disable HTTP keep-alive
				transport.MaxIdleConns = 0         // No idle connections
				transport.MaxIdleConnsPerHost = 0  // No idle connections per host
				transport.IdleConnTimeout = 0      // Immediate timeout for idle connections
				return transport
			}
			// If it's not an *http.Transport, return it as-is
			return rt
		}

		rc, err = rest.RESTClientFor(cfg)
		fmt.Println(rc.Client.Transport)
	}

	c, cancel := context.WithCancel(ctx)
	// If something went wrong or we didn't create a REST client, run without health checking
	if err != nil || rc == nil {
		// I think this scenario is impossible outside testing.
		r.log.WithError(err).Error(fmt.Sprintf("Unable to create restClient for %q healthCheck. Running without healthCheck", addr), err)
		hc := &healthChecker{
			healthy: true,
			cancel:  cancel,
		}
		r.updateCh <- struct{}{}
		return hc
	}

	if healthCheckFn == nil {
		healthCheckFn = func(ctx context.Context) restResult {
			return rc.Get().AbsPath("/healthz").Do(ctx)
		}
	}
	hc := &healthChecker{
		rc:            rc,
		log:           r.log,
		cancel:        cancel,
		updateCh:      r.updateCh,
		healthCheckFn: healthCheckFn,
	}
	go hc.runHealthCheck(c, addr, nil)
	return hc
}

func (hc *healthChecker) Stop() {
	hc.cancel()
}

func (hc *healthChecker) runHealthCheck(ctx context.Context, addr string, c <-chan time.Time) {
	if c == nil {
		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()
		c = ticker.C
	}

	for {
		select {
		case <-ctx.Done():
			hc.log.Infof("Stopping healthchecker for %q", addr)
			hc.healthy = false
			return
		case <-c:
			result := hc.healthCheckFn(ctx)
			hc.mu.Lock()
			if err := result.Error(); err != nil {
				if hc.healthy {
					hc.log.Infof("Health check failed, %q became inactive", addr)
					hc.healthy = false
					hc.updateCh <- struct{}{}
				}
			} else {
				if !hc.healthy {
					hc.log.Infof("Health check succeeded, %q became active", addr)
					hc.healthy = true
					hc.updateCh <- struct{}{}
				}
			}
			hc.mu.Unlock()
		}
	}
}

// restResult is an interface to abstract rest.Result for testing purposes
type restResult interface {
	Error() error
}
