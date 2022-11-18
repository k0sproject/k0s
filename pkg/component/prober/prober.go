/*
Copyright 2022 k0s authors

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package prober

import (
	"container/ring"
	"context"
	"time"

	"github.com/sirupsen/logrus"
)

// Healthz represents a component that can be checked for its health.
type Healthz interface {
	// Healthy performs a periodical health check and indicates that a component is
	// healthy and performs well
	Healthy() error
}

// Prober performs health probes on registred components
type Prober struct {
	l                    *logrus.Entry
	interval             time.Duration
	withHealthComponents map[string]Healthz
	withEventComponents  map[string]Eventer

	// mostly for the test purposes
	stopAfterIterationNum int
	probesTrackLength     int
	healthCheckProbesCh   chan (ProbeResult)
	state                 map[string]*ring.Ring
}

// New creates a new prober
func New() *Prober {
	return &Prober{
		l:                    logrus.WithFields(logrus.Fields{"component": "prober"}),
		interval:             10 * time.Second,
		withHealthComponents: make(map[string]Healthz),
		withEventComponents:  make(map[string]Eventer),
		probesTrackLength:    10,
		state:                make(map[string]*ring.Ring),
		// channel is created before starting the loop to know the buffer size
		healthCheckProbesCh: nil,
	}
}

// State gives read-only copy of current state
func (p *Prober) State() map[string][]ProbeResult {
	state := make(map[string][]ProbeResult)
	for name, r := range p.state {
		state[name] = make([]ProbeResult, 0, p.probesTrackLength)
		r.Do(func(v interface{}) {
			if v == nil {
				return
			}
			state[name] = append(state[name], v.(ProbeResult))
		})
	}
	return state
}

// Run starts the prober workin loop
func (p *Prober) Run(ctx context.Context) {
	p.healthCheckProbesCh = make(chan ProbeResult, len(p.withHealthComponents))
	ticker := time.NewTicker(p.interval)
	p.initRings()
	go p.probesSaveLoop(ctx)
	epoch := 0
	for {
		select {
		case <-ctx.Done():
			return
		case at := <-ticker.C:
			p.l.Info("running health checks")
			p.checkComponentsHealth(ctx, at)
			if p.stopAfterIterationNum > 0 {
				epoch++
				if epoch > p.stopAfterIterationNum {
					return
				}
			}
		}
	}
}

func (p *Prober) initRings() {
	for name := range p.withHealthComponents {
		p.state[name] = ring.New(p.probesTrackLength)
	}
}

func (p *Prober) probesSaveLoop(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case probe := <-p.healthCheckProbesCh:
			p.state[probe.Component].Value = probe
			p.state[probe.Component] = p.state[probe.Component].Next()
		}
	}
}

func (p *Prober) checkComponentsHealth(ctx context.Context, at time.Time) {
	for name, component := range p.withHealthComponents {
		p.healthCheckProbesCh <- ProbeResult{
			Component: name,
			At:        at,
			Error:     component.Healthy(),
		}
	}
}

// Register registers a component to be probed
func (p *Prober) Register(name string, component any) {
	l := p.l.WithField("component", name)

	withHealth, ok := component.(Healthz)
	if ok {
		l.Warnf("component implements Healthz interface, observing")
		p.withHealthComponents[name] = withHealth
	}

	withEvents, ok := component.(Eventer)
	if ok {
		l.Warnf("component implements Eventer interface, subscribing")
		p.withEventComponents[name] = withEvents
	}
}

// ProbeResult represents a result of a probe
type ProbeResult struct {
	Component string
	At        time.Time
	Error     error
}
