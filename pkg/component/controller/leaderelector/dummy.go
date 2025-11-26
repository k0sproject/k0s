// SPDX-FileCopyrightText: 2022 k0s authors
// SPDX-License-Identifier: Apache-2.0

package leaderelector

import (
	"context"

	"github.com/k0sproject/k0s/pkg/leaderelection"
)

// AlwaysLeading is a dummy leader elector that reports itself as the leader
// forever. Used single-node setups where leader election is unnecessary.
type AlwaysLeading struct {
	never     <-chan struct{}
	callbacks []func()
}

// Off returns an always-leading leader elector.
func Off() *AlwaysLeading {
	return &AlwaysLeading{make(<-chan struct{}), nil}
}

func (*AlwaysLeading) Init(context.Context) error {
	return nil
}

func (l *AlwaysLeading) AddAcquiredLeaseCallback(fn func()) {
	l.callbacks = append(l.callbacks, fn)
}

func (*AlwaysLeading) AddLostLeaseCallback(func()) {
}

func (l *AlwaysLeading) Start(context.Context) error {
	for _, fn := range l.callbacks {
		fn()
	}
	return nil
}

func (*AlwaysLeading) IsLeader() bool { return true }

func (l *AlwaysLeading) CurrentStatus() (leaderelection.Status, <-chan struct{}) {
	return leaderelection.StatusLeading, l.never
}

func (*AlwaysLeading) Stop() error {
	return nil
}
