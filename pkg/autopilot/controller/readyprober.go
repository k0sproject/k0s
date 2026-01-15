//go:build unix

// SPDX-FileCopyrightText: 2021 k0s authors
// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"cmp"
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"sync"
	"time"

	apv1beta2 "github.com/k0sproject/k0s/pkg/apis/autopilot/v1beta2"
	k0sclientset "github.com/k0sproject/k0s/pkg/client/clientset"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/sirupsen/logrus"
	k8sprobe "k8s.io/kubernetes/pkg/probe"
	k8shttpprobe "k8s.io/kubernetes/pkg/probe/http"
)

const readyzURLFormat = "https://%s/readyz?verbose"

var (
	// Indicates that a readiness probe failed figure out its target.
	errReadyProbeTargetResolutionFailed = errors.New("target resolution failed")

	// Indicates that a readiness probe has been performed, but the result was
	// not successful.
	errUnsuccessfulReadyProbe = errors.New("ready probe didn't indicate success")
)

type readyProber struct {
	k8sAPIPort int
	log        *logrus.Entry
	tlsConfig  *tls.Config
	timeout    time.Duration
	client     k0sclientset.Interface
}

// Creates a new readyProber.
func newReadyProber(logger *logrus.Entry, client k0sclientset.Interface, tlsConfig *tls.Config, k8sAPIPort int, timeout time.Duration) *readyProber {
	return &readyProber{
		log:        logger,
		client:     client,
		tlsConfig:  tlsConfig,
		timeout:    timeout,
		k8sAPIPort: cmp.Or(k8sAPIPort, 6443),
	}
}

// Runs probes for all targets concurrently and returns the most severe error,
// prioritizing failed target resolutions and canceling the rest.
func (p *readyProber) probeTargets(ctx context.Context, targets []apv1beta2.PlanCommandTargetStatus) error {
	var (
		wg     sync.WaitGroup
		mu     sync.Mutex
		result error
	)

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	for _, target := range targets {
		wg.Add(1)
		go func() {
			defer wg.Done()
			err := p.probeOne(ctx, target)
			if err == nil {
				return
			}

			targetResolutionFailed := errors.Is(err, errReadyProbeTargetResolutionFailed)
			if targetResolutionFailed {
				cancel() // Be fail-fast
			}

			mu.Lock()
			defer mu.Unlock()
			if result == nil || (targetResolutionFailed && !errors.Is(result, errReadyProbeTargetResolutionFailed)) {
				result = fmt.Errorf("while probing %s: %w", target.Name, err)
			}
		}()
	}

	wg.Wait()
	return result
}

// probeOne will lookup the IP address of a target, and then proceed to query a
// well-known endpoint for service readiness.
func (p readyProber) probeOne(ctx context.Context, target apv1beta2.PlanCommandTargetStatus) error {
	p.log.Infof("Probing %v", target.Name)

	ctx, cancel := context.WithTimeout(ctx, p.timeout)
	defer cancel()
	controlnode, err := p.client.AutopilotV1beta2().ControlNodes().Get(ctx, target.Name, metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			return fmt.Errorf("%w: no such ControlNode: %w", errReadyProbeTargetResolutionFailed, err)
		}

		return fmt.Errorf("failed to get ControlNode: %w", err)
	}

	address := controlnode.Status.GetInternalIP()
	if address == "" {
		return errors.New("no internal IP address found for ControlNode")
	}

	probe := k8shttpprobe.NewWithTLSConfig(p.tlsConfig, false /* followNonLocalRedirects */)
	url := fmt.Sprintf(readyzURLFormat, net.JoinHostPort(address, strconv.Itoa(p.k8sAPIPort)))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("unable to create HTTP request for %s:%d: %w", address, p.k8sAPIPort, err)
	}
	// The body content is not interesting at the moment.
	res, msg, err := probe.Probe(req, p.timeout)
	if err != nil {
		return fmt.Errorf("failed to probe %s: %w", url, err)
	}
	if res != k8sprobe.Success {
		return fmt.Errorf("%w: %s: %s", errUnsuccessfulReadyProbe, res, msg)
	}

	p.log.Infof("Probing %s done: %v", target.Name, res)
	return nil
}
