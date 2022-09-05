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

package controller

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"

	"github.com/k0sproject/k0s/pkg/component"
	"github.com/k0sproject/k0s/pkg/leaderelection"
	"github.com/sirupsen/logrus"
)

// LeaderElector is the common leader elector component to manage each controller leader status
type LeaderElector interface {
	IsLeader() bool
	AddCallback(string, LeaseCallback)
	RemoveCallback(string)
}

// LeaseCallback acquired and lost lease callbacks
type LeaseCallback struct {
	OnAcquired CallbackFn
	OnLost     CallbackFn
}

type getPoolFn func() (LeasePoolWatcher, error)
type LeasePoolLeaderElector struct {
	log                     *logrus.Entry
	newComponentRegistrated chan registrationEvent
	componentDeregistrated  chan registrationEvent
	leaderStatus            atomic.Value
	leaseCancel             context.CancelFunc
	getPool                 getPoolFn
	callbacks               map[string]LeaseCallback
	callbacksLock           sync.Mutex
}

type registrationEvent struct {
	Name     string
	Callback CallbackFn
}

type CallbackFn func()

var _ LeaderElector = (*LeasePoolLeaderElector)(nil)
var _ component.Component = (*LeasePoolLeaderElector)(nil)

// NewLeasePoolLeaderElector creates new leader elector using a Kubernetes lease pool.
func NewLeasePoolLeaderElector(getPool getPoolFn) *LeasePoolLeaderElector {
	d := atomic.Value{}
	d.Store(false)
	return &LeasePoolLeaderElector{
		log:                     logrus.WithFields(logrus.Fields{"component": "endpointreconciler"}),
		leaderStatus:            d,
		getPool:                 getPool,
		newComponentRegistrated: make(chan registrationEvent),
		componentDeregistrated:  make(chan registrationEvent),
		callbacks:               make(map[string]LeaseCallback),
	}
}

func (l *LeasePoolLeaderElector) Init(_ context.Context) error {
	return nil
}

type LeasePoolWatcher interface {
	Watch(...leaderelection.WatchOpt) (*leaderelection.LeaseEvents, context.CancelFunc, error)
}

func (l *LeasePoolLeaderElector) Start(ctx context.Context) error {
	pool, err := l.getPool()
	if err != nil {
		return fmt.Errorf("can't get lease pool: %w", err)
	}
	events, cancel, err := pool.Watch()
	if err != nil {
		return fmt.Errorf("can't start watching lease pool: %w", err)
	}
	l.leaseCancel = cancel

	go func() {
		for {
			select {
			case <-events.AcquiredLease:
				l.log.Info("acquired leader lease")
				l.leaderStatus.Store(true)
				go l.runCallbacks(acquiredEventType, l.callbacks)
			case <-events.LostLease:
				l.log.Info("lost leader lease")
				l.leaderStatus.Store(false)
				go l.runCallbacks(lostEventType, l.callbacks)
			case c := <-l.newComponentRegistrated:
				go l.runCallbacks(acquiredEventType, map[string]LeaseCallback{
					c.Name: {OnAcquired: c.Callback},
				})
			case c := <-l.componentDeregistrated:
				go l.runCallbacks(lostEventType, map[string]LeaseCallback{
					c.Name: {OnLost: c.Callback},
				})
			}
		}
	}()
	return nil
}

type leaseEventType int

const acquiredEventType = 1
const lostEventType = 2

func (l *LeasePoolLeaderElector) runCallbacks(event leaseEventType, callbacks map[string]LeaseCallback) {
	var wg sync.WaitGroup
	l.callbacksLock.Lock()
	for _, cb := range callbacks {
		var fn CallbackFn
		switch event {
		case acquiredEventType:
			fn = cb.OnAcquired
		case lostEventType:
			fn = cb.OnLost
		}
		if fn != nil {
			wg.Add(1)
			go func() {
				fn()
				wg.Done()
			}()
		}
	}
	l.callbacksLock.Unlock()
	wg.Wait()
}

func (l *LeasePoolLeaderElector) AddCallback(name string, cb LeaseCallback) {
	l.callbacksLock.Lock()
	l.callbacks[name] = cb
	l.callbacksLock.Unlock()
	if l.IsLeader() && cb.OnAcquired != nil {
		// schedpe ule new callback for run if the current instance is leader
		l.newComponentRegistrated <- registrationEvent{
			Name:     name,
			Callback: cb.OnAcquired,
		}
	}
}

func (l *LeasePoolLeaderElector) RemoveCallback(name string) {
	l.callbacksLock.Lock()
	cb := l.callbacks[name]
	delete(l.callbacks, name)
	l.callbacksLock.Unlock()
	if l.IsLeader() && cb.OnLost != nil {
		l.componentDeregistrated <- registrationEvent{
			Name:     name,
			Callback: cb.OnLost,
		}
	}
}

func (l *LeasePoolLeaderElector) Stop() error {
	if l.leaseCancel != nil {
		l.leaseCancel()
	}
	return nil
}

func (l *LeasePoolLeaderElector) IsLeader() bool {
	return l.leaderStatus.Load().(bool)
}
