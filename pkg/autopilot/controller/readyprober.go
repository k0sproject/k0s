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
	"context"
	"crypto/tls"
	"fmt"
	"net/http"
	"time"

	apv1beta2 "github.com/k0sproject/k0s/pkg/apis/autopilot/v1beta2"
	apcli "github.com/k0sproject/k0s/pkg/autopilot/client"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/sirupsen/logrus"
	"golang.org/x/sync/errgroup"
	"k8s.io/client-go/rest"
	k8sprobe "k8s.io/kubernetes/pkg/probe"
	k8shttpprobe "k8s.io/kubernetes/pkg/probe/http"
)

const readyzURLFormat = "https://%s:6443/readyz?verbose"

type ReadyProber interface {
	Probe() error
	AddTargets(targets []apv1beta2.PlanCommandTargetStatus)
}

type readyProber struct {
	log           *logrus.Entry
	tlsConfig     *tls.Config
	timeout       time.Duration
	targets       []apv1beta2.PlanCommandTargetStatus
	clientFactory apcli.FactoryInterface
}

// NewReadyProber creates a new ReadyProber based on a REST configuration, and is
// populated with PlanCommandTargetStatus targets assigned via AddTargets.
func NewReadyProber(logger *logrus.Entry, cf apcli.FactoryInterface, restConfig *rest.Config, timeout time.Duration) (ReadyProber, error) {
	tlscfg, err := rest.TLSConfigFor(restConfig)
	if err != nil {
		return nil, err
	}

	return &readyProber{
		log:           logger,
		clientFactory: cf,
		tlsConfig:     tlscfg,
		timeout:       timeout,
	}, nil
}

// AddTargets adds all of the `PlanCommandTargetStatus` targets that should
// be probed into the prober.
func (p *readyProber) AddTargets(targets []apv1beta2.PlanCommandTargetStatus) {
	p.targets = targets
}

// Probe starts goroutines for each of the provided targets and starts their probe.
// As errors are received, they are collected in a single errors channel for post
// inspection. This function blocks until *all* spawned goroutines have completed
// or timed-out.
func (p readyProber) Probe() error {
	g := errgroup.Group{}

	for _, target := range p.targets {
		g.Go(func() error { return p.probeOne(target) })
	}

	return g.Wait()
}

// probeOne will lookup the IP address of a target, and then proceed to query a
// well-known endpoint for service readiness.
func (p readyProber) probeOne(target apv1beta2.PlanCommandTargetStatus) error {
	p.log.Infof("Probing %v", target.Name)

	client, err := p.clientFactory.GetK0sClient()
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), p.timeout)
	defer cancel()
	controlnode, err := client.AutopilotV1beta2().ControlNodes().Get(ctx, target.Name, metav1.GetOptions{})
	if err != nil {
		return err
	}

	address := controlnode.Status.GetInternalIP()
	if address == "" {
		return fmt.Errorf("no internal IP address found for %s", target.Name)
	}

	probe := k8shttpprobe.NewWithTLSConfig(p.tlsConfig, false /* followNonLocalRedirects */)
	url := fmt.Sprintf(readyzURLFormat, address)
	req, err := http.NewRequest(http.MethodGet, url, nil)
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
