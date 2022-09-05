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
	"testing"

	"github.com/k0sproject/k0s/pkg/leaderelection"
	"github.com/stretchr/testify/require"
)

type fakePoolWatcher struct {
	events *leaderelection.LeaseEvents
}

func (p *fakePoolWatcher) Watch(...leaderelection.WatchOpt) (*leaderelection.LeaseEvents, context.CancelFunc, error) {
	p.events = &leaderelection.LeaseEvents{
		AcquiredLease: make(chan struct{}),
		LostLease:     make(chan struct{}),
	}
	go func() {
		p.events.AcquiredLease <- struct{}{}
	}()

	return p.events, nil, nil
}

func (p *fakePoolWatcher) lose() {
	go func() {
		p.events.LostLease <- struct{}{}
	}()
}

func TestLeaderElector(t *testing.T) {
	t.Run("acquired_and_lost_callback_called_if_registrated_before_starting_working_loop", func(t *testing.T) {
		firstCalled := make(chan bool)
		firstStopped := make(chan bool)
		ctx := context.Background()
		watcher := &fakePoolWatcher{}
		elector := NewLeasePoolLeaderElector(func() (LeasePoolWatcher, error) {
			return watcher, nil
		})
		elector.AddCallback("first", LeaseCallback{
			OnAcquired: func() {
				firstCalled <- true
			},
			OnLost: func() {
				firstStopped <- true
			},
		})

		require.NoError(t, elector.Start(ctx))
		watcher.lose()
		require.True(t, <-firstCalled, "Must call all start callbacks registrated before start")
		require.True(t, <-firstStopped, "Must call all lost callbacks registrated before start")
	})

	t.Run("acquired_and_lost_callback_called_if_registrated_after_starting_working_loop", func(t *testing.T) {
		firstCalled := make(chan bool)
		firstStopped := make(chan bool)
		ctx := context.Background()
		watcher := &fakePoolWatcher{}
		elector := NewLeasePoolLeaderElector(func() (LeasePoolWatcher, error) {
			return watcher, nil
		})

		require.NoError(t, elector.Start(ctx))
		elector.AddCallback("first", LeaseCallback{
			OnAcquired: func() {
				firstCalled <- true
			},
			OnLost: func() {
				firstStopped <- true
			},
		})
		watcher.lose()
		require.True(t, <-firstCalled, "Must call all start callbacks registrated before start")
		require.True(t, <-firstStopped, "Must call all lost callbacks registrated before start")
	})

	t.Run("lost_callback_is_called_on_deregestritration_even_if_elector_is_still_leader", func(t *testing.T) {
		firstCalled := make(chan bool)
		firstStopped := make(chan bool)
		ctx := context.Background()
		watcher := &fakePoolWatcher{}
		elector := NewLeasePoolLeaderElector(func() (LeasePoolWatcher, error) {
			return watcher, nil
		})
		elector.AddCallback("first", LeaseCallback{
			OnAcquired: func() {
				firstCalled <- true
			},
			OnLost: func() {
				firstStopped <- true
			},
		})
		require.NoError(t, elector.Start(ctx))
		require.True(t, <-firstCalled, "Must call all start callbacks registrated before start")
		elector.RemoveCallback("first")
		require.True(t, <-firstStopped, "Must call all lost callbacks registrated before start")
		require.True(t, elector.IsLeader(), "elector is still leader")
	})
}
