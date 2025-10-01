// SPDX-FileCopyrightText: 2022 k0s authors
// SPDX-License-Identifier: Apache-2.0

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
