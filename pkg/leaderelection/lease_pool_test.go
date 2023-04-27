/*
Copyright 2020 k0s authors

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

package leaderelection

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	coordinationv1 "k8s.io/api/coordination/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
	fakecoordinationv1 "k8s.io/client-go/kubernetes/typed/coordination/v1/fake"
	k8stesting "k8s.io/client-go/testing"
	"k8s.io/utils/pointer"
)

func TestLeasePoolWatcherTriggersOnLeaseAcquisition(t *testing.T) {
	const identity = "test-node"

	fakeClient := fake.NewSimpleClientset()

	pool, err := NewLeasePool(context.TODO(), fakeClient, "test", WithIdentity(identity), WithNamespace("test"))
	require.NoError(t, err)

	output := &LeaseEvents{
		AcquiredLease: make(chan struct{}, 1),
		LostLease:     make(chan struct{}, 1),
	}

	events, cancel, err := pool.Watch(WithOutputChannels(output))
	require.NoError(t, err)
	defer cancel()

	done := make(chan struct{})
	failed := make(chan struct{})

	go func() {
		for {
			select {
			case <-events.AcquiredLease:
				close(done)
			case <-events.LostLease:
				close(failed)
			}
		}
	}()

	select {
	case <-done:
		t.Log("successfully acquired lease")
	case <-failed:
		assert.Fail(t, "lost lease")
	}
}

func TestLeasePoolTriggersLostLeaseWhenCancelled(t *testing.T) {
	const identity = "test-node"

	fakeClient := fake.NewSimpleClientset()

	pool, err := NewLeasePool(context.TODO(), fakeClient, "test", WithIdentity(identity), WithNamespace("test"))
	require.NoError(t, err)

	output := &LeaseEvents{
		AcquiredLease: make(chan struct{}, 1),
		LostLease:     make(chan struct{}, 1),
	}

	events, cancel, err := pool.Watch(WithOutputChannels(output))
	require.NoError(t, err)

	<-events.AcquiredLease
	t.Log("lease acquired, cancelling leaser")
	cancel()
	<-events.LostLease
	t.Log("context cancelled and lease successfully lost")
}

func TestLeasePoolWatcherReacquiresLostLease(t *testing.T) {
	const identity = "test-node"

	fakeClient := fake.NewSimpleClientset()

	givenLeaderElectorError := func() func(err error) {
		var updateErr atomic.Value
		fakeClient.CoordinationV1().(*fakecoordinationv1.FakeCoordinationV1).PrependReactor("update", "leases", func(action k8stesting.Action) (bool, runtime.Object, error) {
			if err := *updateErr.Load().(*error); err != nil {
				return true, nil, err
			}
			return false, nil, nil
		})

		return func(err error) {
			updateErr.Store(&err)
		}
	}()

	pool, err := NewLeasePool(context.TODO(), fakeClient, "test",
		WithIdentity(identity), WithNamespace("test"),
		WithRetryPeriod(10*time.Millisecond),
	)
	require.NoError(t, err)

	output := &LeaseEvents{
		AcquiredLease: make(chan struct{}, 1),
		LostLease:     make(chan struct{}, 1),
	}

	givenLeaderElectorError(nil)
	events, cancel, err := pool.Watch(WithOutputChannels(output))
	require.NoError(t, err)
	defer cancel()

	<-events.AcquiredLease
	t.Log("Acquired lease, disrupting leader election and waiting to loose the lease")
	givenLeaderElectorError(errors.New("leader election disrupted by test case"))

	<-events.LostLease
	t.Log("Lost lease, restoring leader election and waiting for the reacquisition of the lease")
	givenLeaderElectorError(nil)

	select {
	case <-events.AcquiredLease:
		t.Log("Reacquired lease, all good ...")
	case <-time.After(10 * time.Second):
		assert.Fail(t, "Timed out while waiting for the reacquisition of the lease")
	}
}

func TestSecondWatcherAcquiresReleasedLease(t *testing.T) {
	fakeClient := fake.NewSimpleClientset()

	pool1, err := NewLeasePool(context.TODO(), fakeClient, "test",
		WithIdentity("pool1"), WithNamespace("test"),
		WithRetryPeriod(10*time.Millisecond),
	)
	require.NoError(t, err)

	pool2, err := NewLeasePool(context.TODO(), fakeClient, "test",
		WithIdentity("pool2"), WithNamespace("test"),
		WithRetryPeriod(10*time.Millisecond),
	)
	require.NoError(t, err)

	expectedEventOrder := []string{"pool1-acquired", "pool1-lost", "pool2-acquired"}

	// Pre-create the acquired lease for the first identity, so that there are
	// no races when acquiring the lease by the two competing pools.
	now := metav1.NewMicroTime(time.Now())
	_, err = fakeClient.CoordinationV1().Leases("test").Create(context.TODO(), &coordinationv1.Lease{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Lease",
			APIVersion: coordinationv1.SchemeGroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: "test",
		},
		Spec: coordinationv1.LeaseSpec{
			HolderIdentity:       pointer.String("pool1"),
			AcquireTime:          &now,
			RenewTime:            &now,
			LeaseDurationSeconds: pointer.Int32(60 * 60), // block lease for a very long time
		},
	}, metav1.CreateOptions{})
	require.NoError(t, err)
	t.Log("Pre-created acquired lease for first identity")

	events1, cancel1, err := pool1.Watch(WithOutputChannels(&LeaseEvents{
		AcquiredLease: make(chan struct{}, 1),
		LostLease:     make(chan struct{}, 1),
	}))
	require.NoError(t, err)
	defer cancel1()
	t.Log("Started first lease pool")

	events2, cancel2, err := pool2.Watch(WithOutputChannels(&LeaseEvents{
		AcquiredLease: make(chan struct{}, 1),
		LostLease:     make(chan struct{}, 1),
	}))
	require.NoError(t, err)
	defer cancel2()
	t.Log("Started second lease pool, receiving events ...")

	var receivedEvents []string

	for {
		select {
		case <-events1.AcquiredLease:
			t.Log("First lease acquired, cancelling pool")
			cancel1()
			receivedEvents = append(receivedEvents, "pool1-acquired")
		case <-events1.LostLease:
			t.Log("First lease lost")
			receivedEvents = append(receivedEvents, "pool1-lost")
		case <-events2.AcquiredLease:
			t.Log("Second lease acquired")
			receivedEvents = append(receivedEvents, "pool2-acquired")
		case <-events2.LostLease:
			t.Log("Second lease lost")
			receivedEvents = append(receivedEvents, "pool2-lost")
		case <-time.After(10 * time.Second):
			require.Fail(t, "Didn't receive any events for 10 seconds.")
		}

		if len(receivedEvents) >= 3 {
			break
		}
	}

	assert.Equal(t, expectedEventOrder, receivedEvents)
}
