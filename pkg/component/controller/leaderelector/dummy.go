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

package leaderelector

import (
	"context"

	"github.com/k0sproject/k0s/pkg/component/manager"
	"github.com/k0sproject/k0s/pkg/leaderelection"
)

type Dummy struct {
	Leader    bool
	callbacks []func()
}

var _ Interface = (*Dummy)(nil)
var _ manager.Component = (*Dummy)(nil)

func (l *Dummy) Init(_ context.Context) error { return nil }
func (l *Dummy) Stop() error                  { return nil }
func (l *Dummy) IsLeader() bool               { return l.Leader }

func (l *Dummy) AddAcquiredLeaseCallback(fn func()) {
	l.callbacks = append(l.callbacks, fn)
}

var never = make(<-chan struct{})

func (l *Dummy) CurrentStatus() (leaderelection.Status, <-chan struct{}) {
	var status leaderelection.Status
	if l.Leader {
		status = leaderelection.StatusLeading
	}

	return status, never
}

func (l *Dummy) AddLostLeaseCallback(func()) {}

func (l *Dummy) Start(_ context.Context) error {
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
