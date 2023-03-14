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
	"encoding/json"
	"errors"
	"sync"
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
	sync.RWMutex
	l                    *logrus.Entry
	interval             time.Duration
	withHealthComponents map[string]Healthz
	withEventComponents  map[string]Eventer

	probesTrackLength int
	healthCheckState  map[string]*ring.Ring

	closeCh chan struct{}
	startCh chan struct{}
	runOnce sync.Once

	eventsTrackLength int
	eventState        map[string]*ring.Ring
	// mostly for the test purposes
	stopAfterIterationNum int
}

// New creates a new prober
func New() *Prober {
	return &Prober{
		l:                    logrus.WithFields(logrus.Fields{"component": "prober"}),
		interval:             10 * time.Second,
		withHealthComponents: make(map[string]Healthz),
		withEventComponents:  make(map[string]Eventer),
		eventsTrackLength:    3,
		probesTrackLength:    3,
		healthCheckState:     make(map[string]*ring.Ring),
		eventState:           make(map[string]*ring.Ring),
		closeCh:              make(chan struct{}),
		startCh:              make(chan struct{}),
	}
}

// DefaultProber default global instance
var DefaultProber = New()

// State gives read-only copy of current state
func (p *Prober) State(maxCount int) State {
	p.RLock()
	defer p.RUnlock()
	state := State{
		HealthProbes: make(map[string][]ProbeResult),
		Events:       make(map[string][]Event),
	}
	for name, r := range p.healthCheckState {
		maxCount := maxCount
		state.HealthProbes[name] = make([]ProbeResult, 0, p.probesTrackLength*len(p.withHealthComponents))
		r.Do(func(v interface{}) {
			if v == nil {
				return
			}
			state.HealthProbes[name] = append(state.HealthProbes[name], v.(ProbeResult))
		})
		if maxCount >= p.probesTrackLength {
			maxCount = p.probesTrackLength
		}
		if maxCount > len(state.HealthProbes[name]) {
			maxCount = len(state.HealthProbes[name])
		}
		state.HealthProbes[name] = state.HealthProbes[name][0:maxCount]
	}
	for name, r := range p.eventState {
		maxCount := maxCount
		state.Events[name] = make([]Event, 0, p.eventsTrackLength*len(p.withEventComponents))
		r.Do(func(v interface{}) {
			if v == nil {
				return
			}
			state.Events[name] = append(state.Events[name], v.(Event))
		})
		if maxCount >= p.eventsTrackLength {
			maxCount = p.eventsTrackLength
		}
		if maxCount > len(state.Events[name]) {
			maxCount = len(state.Events[name])
		}

		state.Events[name] = state.Events[name][0:maxCount]

	}

	return state
}

type State struct {
	HealthProbes map[string][]ProbeResult `json:"healthProbes"`
	Events       map[string][]Event       `json:"events"`
}

// Run starts the prober workin loop
func (p *Prober) Run(ctx context.Context) {
	p.runOnce.Do(func() {
		close(p.startCh)
		p.healthCheckLoop(ctx)
		close(p.closeCh)
	})
}

func (p *Prober) healthCheckLoop(ctx context.Context) {
	epoch := 0
	ticker := time.NewTicker(p.interval)
	for {
		select {
		case <-ctx.Done():
			return
		case at := <-ticker.C:
			p.l.Debug("Probing components")
			p.checkComponentsHealth(ctx, at)
			// limit amount of iterations for the test purposes
			if p.stopAfterIterationNum > 0 {
				epoch++
				if epoch >= p.stopAfterIterationNum {
					return
				}
			}
		}
	}
}
func (p *Prober) checkComponentsHealth(ctx context.Context, at time.Time) {
	for name, component := range p.withHealthComponents {
		p.Lock()
		if _, ok := p.healthCheckState[name]; !ok {
			p.healthCheckState[name] = ring.New(p.probesTrackLength)
		}
		// TODO: add back-off logic
		p.healthCheckState[name].Value = ProbeResult{
			Component: name,
			At:        at,
			Error:     component.Healthy(),
		}
		p.healthCheckState[name] = p.healthCheckState[name].Next()
		p.Unlock()
	}
}

func (p *Prober) spawnEventCollector(name string, component Eventer) {
	p.Lock()
	p.eventState[name] = ring.New(p.eventsTrackLength)
	p.Unlock()
	go func() {
		<-p.startCh // wait for the start signal
		for {
			select {
			case <-p.closeCh:
				return
			case event := <-component.Events():
				p.l.WithField("component", name).WithField("event", event).Debug("Got event")
				p.Lock()
				p.eventState[name].Value = event
				p.eventState[name] = p.eventState[name].Next()
				p.Unlock()
			}
		}
	}()
}

// Register registers a component to be probed
func (p *Prober) Register(name string, component any) {
	l := p.l.WithField("component", name)

	withHealth, ok := component.(Healthz)
	if ok {
		l.Debug("component implements Healthz interface, observing")
		p.withHealthComponents[name] = withHealth
	}

	withEvents, ok := component.(Eventer)
	if ok {
		l.Debug("component implements Eventer interface, subscribing")
		p.withEventComponents[name] = withEvents
		p.spawnEventCollector(name, withEvents)
	}

}

// ProbeError is a string that implements the error interface.
// This is necessary because errors in golang are an interface and not structs,
// which means they are marshalled as an empty json and cannot be unmarshalled.
type ProbeError string

// ProbeResult represents a result of a probe
type ProbeResult struct {
	Component string
	At        time.Time
	Error     error
}

// probeResultMarshaller is a struct used internally to marshal and unmarshal
// ProbeResults. It's meant to be used only internally.
type probeResultMarshaller struct {
	Component string    `json:"component"`
	At        time.Time `json:"at"`
	Error     string    `json:"error"`
}

func (pr *ProbeResult) MarshalJSON() ([]byte, error) {
	errStr := ""
	if pr.Error != nil {
		errStr = pr.Error.Error()
	}

	prm := &probeResultMarshaller{
		Component: pr.Component,
		At:        pr.At,
		Error:     errStr,
	}

	return json.Marshal(prm)
}

func (pr *ProbeResult) UnmarshalJSON(data []byte) error {
	upr := &probeResultMarshaller{}

	err := json.Unmarshal(data, upr)
	if err != nil {
		return err
	}

	pr.Component = upr.Component
	pr.At = upr.At
	pr.Error = errors.New(upr.Error)
	return nil
}
