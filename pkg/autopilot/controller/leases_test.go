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
	"fmt"
	"testing"
	"time"

	"github.com/k0sproject/k0s/internal/autopilot/testutil"
	"github.com/k0sproject/k0s/pkg/autopilot/constant"
	"github.com/k0sproject/k0s/pkg/leaderelection"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

// TestLeasesInitialPending ensures that when a lease watcher is created,
// the first event received is a 'pending' event.
func TestLeasesInitialPending(t *testing.T) {
	clientFactory := testutil.NewFakeClientFactory()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	logger := logrus.StandardLogger().WithField("app", "leases_test")

	leaseWatcher, err := NewLeaseWatcher(logger, clientFactory)
	assert.NoError(t, err)

	leaseEventStatusCh, errorCh := leaseWatcher.StartWatcher(ctx, constant.AutopilotNamespace, fmt.Sprintf("%s-lease", constant.AutopilotNamespace), t.Name())
	assert.NotNil(t, errorCh)
	assert.NotNil(t, leaseEventStatusCh)

	timer := time.NewTimer(2 * time.Second)
	defer timer.Stop()

	select {
	case <-timer.C:
		assert.Fail(t, "Timed out waiting for LeaseEventStatus")

	case leaseEventStatus, ok := <-leaseEventStatusCh:
		assert.True(t, ok)
		assert.NotEmpty(t, leaseEventStatus)
		assert.Equal(t, LeasePending, leaseEventStatus)
	}
}

func closeLeaseEvents(events *leaderelection.LeaseEvents) {
	close(events.AcquiredLease)
	close(events.LostLease)
}

// TestLeadershipWatcher runs through a table of tests that describe
// various lease acquired/lost scenarios
func TestLeadershipWatcher(t *testing.T) {
	var tests = []struct {
		name           string
		expectedEvents []LeaseEventStatus
		eventSource    func(events *leaderelection.LeaseEvents)
	}{
		{
			"AcquiredThenLost",
			[]LeaseEventStatus{
				LeaseAcquired,
				LeasePending,
			},
			func(events *leaderelection.LeaseEvents) {
				sendEventAfter100ms(events.AcquiredLease)
				sendEventAfter100ms(events.LostLease)
				closeLeaseEvents(events)
			},
		},
		{
			"LostThenAcquired",
			[]LeaseEventStatus{
				LeasePending,
				LeaseAcquired,
			},
			func(events *leaderelection.LeaseEvents) {
				sendEventAfter100ms(events.LostLease)
				sendEventAfter100ms(events.AcquiredLease)
				closeLeaseEvents(events)
			},
		},
		{
			"AcquiredThenLostThenAcquired",
			[]LeaseEventStatus{
				LeaseAcquired,
				LeasePending,
			},
			func(events *leaderelection.LeaseEvents) {
				sendEventAfter100ms(events.AcquiredLease)
				sendEventAfter100ms(events.LostLease)
				sendEventAfter100ms(events.AcquiredLease)
				closeLeaseEvents(events)
			},
		},
		{
			"DoubleLostMakesNoSense",
			[]LeaseEventStatus{
				LeasePending,
			},
			func(events *leaderelection.LeaseEvents) {
				sendEventAfter100ms(events.LostLease)
				closeLeaseEvents(events)
			},
		},
		{
			"DoubleAcquireMakesNoSense",
			[]LeaseEventStatus{
				LeaseAcquired,
			},
			func(events *leaderelection.LeaseEvents) {
				sendEventAfter100ms(events.AcquiredLease)
				sendEventAfter100ms(events.AcquiredLease)
				closeLeaseEvents(events)
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			leaseEventStatusCh := make(chan LeaseEventStatus, 100)

			events := &leaderelection.LeaseEvents{
				AcquiredLease: make(chan struct{}),
				LostLease:     make(chan struct{}),
			}

			go test.eventSource(events)

			ctx, cancel := context.WithDeadline(context.TODO(), time.Now().Add(1*time.Second))
			wg := leadershipWatcher(ctx, leaseEventStatusCh, events)
			wg.Wait()

			close(leaseEventStatusCh)
			cancel()

			assert.Equal(t, test.expectedEvents, realizeLeaseEventStatus(leaseEventStatusCh))
		})
	}
}

func realizeLeaseEventStatus(ch chan LeaseEventStatus) []LeaseEventStatus {
	s := make([]LeaseEventStatus, 0)
	for ev := range ch {
		s = append(s, ev)
	}
	return s
}

func sendEventAfter100ms(out chan struct{}) {
	time.Sleep(100 * time.Millisecond)
	out <- struct{}{}
}
