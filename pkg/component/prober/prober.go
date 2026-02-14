// SPDX-FileCopyrightText: 2022 k0s authors
// SPDX-License-Identifier: Apache-2.0

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

const (
	healthCheckInterval = 10 * time.Second
	probesTrackLength   = 3
	eventsTrackLength   = 3
)

// Prober performs health probes on registered components
type Prober struct {
	sync.RWMutex
	l                    *logrus.Entry
	withHealthComponents map[string]Healthz
	withEventComponents  map[string]Eventer

	healthCheckState map[string]*ring.Ring

	closeCh chan struct{}
	startCh chan struct{}
	runOnce sync.Once

	eventState map[string]*ring.Ring
}

// New creates a new prober
func New() *Prober {
	return &Prober{
		l:                    logrus.WithFields(logrus.Fields{"component": "prober"}),
		withHealthComponents: make(map[string]Healthz),
		withEventComponents:  make(map[string]Eventer),
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
		state.HealthProbes[name] = make([]ProbeResult, 0, probesTrackLength*len(p.withHealthComponents))
		r.Do(func(v any) {
			if v == nil {
				return
			}
			state.HealthProbes[name] = append(state.HealthProbes[name], v.(ProbeResult))
		})
		if maxCount >= probesTrackLength {
			maxCount = probesTrackLength
		}
		if maxCount > len(state.HealthProbes[name]) {
			maxCount = len(state.HealthProbes[name])
		}
		state.HealthProbes[name] = state.HealthProbes[name][0:maxCount]
	}
	for name, r := range p.eventState {
		maxCount := maxCount
		state.Events[name] = make([]Event, 0, eventsTrackLength*len(p.withEventComponents))
		r.Do(func(v any) {
			if v == nil {
				return
			}
			state.Events[name] = append(state.Events[name], v.(Event))
		})
		if maxCount >= eventsTrackLength {
			maxCount = eventsTrackLength
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

// Run starts the prober working loop
func (p *Prober) Run(ctx context.Context) {
	p.runOnce.Do(func() {
		close(p.startCh)
		p.healthCheckLoop(ctx)
		close(p.closeCh)
	})
}

func (p *Prober) healthCheckLoop(ctx context.Context) {
	ticker := time.NewTicker(healthCheckInterval)
	for {
		select {
		case <-ctx.Done():
			return
		case at := <-ticker.C:
			p.l.Debug("Probing components")
			p.checkComponentsHealth(at)
		}
	}
}
func (p *Prober) checkComponentsHealth(at time.Time) {
	for name, component := range p.withHealthComponents {
		p.Lock()
		if _, ok := p.healthCheckState[name]; !ok {
			p.healthCheckState[name] = ring.New(probesTrackLength)
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
	p.eventState[name] = ring.New(eventsTrackLength)
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
// which means they are marshaled as an empty json and cannot be unmarshalled.
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
