//go:build unix

// Copyright 2021 k0s authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package controller

import (
	"cmp"
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"time"

	apv1beta2 "github.com/k0sproject/k0s/pkg/apis/autopilot/v1beta2"
	k0sclientset "github.com/k0sproject/k0s/pkg/client/clientset"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/sirupsen/logrus"
	"golang.org/x/sync/errgroup"
	k8sprobe "k8s.io/kubernetes/pkg/probe"
	k8shttpprobe "k8s.io/kubernetes/pkg/probe/http"
)

const readyzURLFormat = "https://%s/readyz?verbose"

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

// Probes the given targets concurrently. Returns the first target probe error
// that is encountered, if any.
func (p *readyProber) probeTargets(ctx context.Context, targets []apv1beta2.PlanCommandTargetStatus) error {
	g, ctx := errgroup.WithContext(ctx)

	for _, target := range targets {
		g.Go(func() error { return p.probeOne(ctx, target) })
	}

	return g.Wait()
}

// probeOne will lookup the IP address of a target, and then proceed to query a
// well-known endpoint for service readiness.
func (p readyProber) probeOne(ctx context.Context, target apv1beta2.PlanCommandTargetStatus) error {
	p.log.Infof("Probing %v", target.Name)

	ctx, cancel := context.WithTimeout(ctx, p.timeout)
	defer cancel()
	controlnode, err := p.client.AutopilotV1beta2().ControlNodes().Get(ctx, target.Name, metav1.GetOptions{})
	if err != nil {
		return err
	}

	address := controlnode.Status.GetInternalIP()
	if address == "" {
		return fmt.Errorf("no internal IP address found for %s", target.Name)
	}

	probe := k8shttpprobe.NewWithTLSConfig(p.tlsConfig, false /* followNonLocalRedirects */)
	url := fmt.Sprintf(readyzURLFormat, net.JoinHostPort(address, strconv.Itoa(p.k8sAPIPort)))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("unable to create HTTP request for '%s': %w", url, err)
	}
	// The body content is not interesting at the moment.
	res, _, err := probe.Probe(req, p.timeout)
	if err != nil {
		return fmt.Errorf("failed to HTTP probe '%s/%s': %w", target.Name, address, err)
	}

	if res != k8sprobe.Success {
		return fmt.Errorf("failed to probe '%s/%s': result=%v", target.Name, address, res)
	}

	p.log.Infof("Probing %s done: %v", target.Name, res)
	return nil
}
