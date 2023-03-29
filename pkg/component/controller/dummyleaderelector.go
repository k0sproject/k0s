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

	"github.com/k0sproject/k0s/pkg/component"
)

type DummyLeaderElector struct {
	Leader    bool
	callbacks []func()
}

var _ LeaderElector = (*DummyLeaderElector)(nil)
var _ component.Component = (*DummyLeaderElector)(nil)

func (l *DummyLeaderElector) Init(_ context.Context) error { return nil }
func (l *DummyLeaderElector) Stop() error                  { return nil }
func (l *DummyLeaderElector) IsLeader() bool               { return l.Leader }
func (l *DummyLeaderElector) Healthy() error               { return nil }

func (l *DummyLeaderElector) AddAcquiredLeaseCallback(fn func()) {
	l.callbacks = append(l.callbacks, fn)
}

func (l *DummyLeaderElector) AddLostLeaseCallback(func()) {}

func (l *DummyLeaderElector) Run(_ context.Context) error {
	if !l.Leader {
		return nil
	}
	for _, fn := range l.callbacks {
		if fn != nil {
			fn()
		}
	}
	return nil
}
