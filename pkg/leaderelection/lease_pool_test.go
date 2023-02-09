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
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
	fakecoordinationv1 "k8s.io/client-go/kubernetes/typed/coordination/v1/fake"
	k8stesting "k8s.io/client-go/testing"
)

func TestLeasePoolWatcherTriggersOnLeaseAcquisition(t *testing.T) {
	const identity = "test-node"

	fakeClient := fake.NewSimpleClientset()
	expectCreateNamespace(t, fakeClient)
	expectCreateLease(t, fakeClient, identity)

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
	expectCreateNamespace(t, fakeClient)
	expectCreateLease(t, fakeClient, identity)

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
	expectCreateNamespace(t, fakeClient)
	expectCreateLease(t, fakeClient, identity)

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
		WithRetryPeriod(350*time.Millisecond),
		WithRenewDeadline(500*time.Millisecond),
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
	const (
		identity  = "test-node"
		identity2 = "test-node-2"
	)

	fakeClient := fake.NewSimpleClientset()
	expectCreateNamespace(t, fakeClient)
	expectCreateLease(t, fakeClient, identity)

	pool, err := NewLeasePool(context.TODO(), fakeClient, "test",
		WithIdentity(identity), WithNamespace("test"),
		WithRetryPeriod(350*time.Millisecond),
		WithRenewDeadline(500*time.Millisecond),
	)

	require.NoError(t, err)

	events, cancel, err := pool.Watch(WithOutputChannels(&LeaseEvents{
		AcquiredLease: make(chan struct{}, 1),
		LostLease:     make(chan struct{}, 1),
	}))
	require.NoError(t, err)
	defer cancel()

	pool2, err := NewLeasePool(context.TODO(), fakeClient, "test",
		WithIdentity(identity2),
		WithNamespace("test"),
		WithRetryPeriod(350*time.Millisecond),
		WithRenewDeadline(500*time.Millisecond),
	)
	require.NoError(t, err)

	events2, cancel2, err := pool2.Watch(WithOutputChannels(&LeaseEvents{
		AcquiredLease: make(chan struct{}, 1),
		LostLease:     make(chan struct{}, 1),
	}))
	require.NoError(t, err)
	defer cancel2()
	t.Log("started second lease holder")

	var receivedEvents []string

leaseEventLoop:
	for {
		select {
		case <-events.AcquiredLease:
			t.Log("lease acquired, cancelling leaser")
			cancel()
			receivedEvents = append(receivedEvents, "node1-acquired")
		case <-events.LostLease:
			t.Log("context cancelled and node 1 lease successfully lost")
			receivedEvents = append(receivedEvents, "node1-lost")
		case <-events2.AcquiredLease:
			t.Log("node 2 lease acquired")
			receivedEvents = append(receivedEvents, "node2-acquired")
		default:
			if len(receivedEvents) >= 3 {
				break leaseEventLoop
			}
		}
	}

	assert.Equal(t, "node1-acquired", receivedEvents[0])
	assert.Equal(t, "node1-lost", receivedEvents[1])
	assert.Equal(t, "node2-acquired", receivedEvents[2])
}

func expectCreateNamespace(t *testing.T, fakeClient *fake.Clientset) {
	_, err := fakeClient.CoreV1().Namespaces().Create(context.TODO(), &v1.Namespace{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Namespace",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: "test",
		},
		Spec: v1.NamespaceSpec{},
	}, metav1.CreateOptions{})
	require.NoError(t, err)
}

func expectCreateLease(t *testing.T, fakeClient *fake.Clientset, identity string) {
	_, err := fakeClient.CoordinationV1().Leases("test").Create(context.TODO(), &coordinationv1.Lease{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Lease",
			APIVersion: "coordination.k8s.io/v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: "test",
		},
		Spec: coordinationv1.LeaseSpec{
			HolderIdentity: &identity,
		},
	}, metav1.CreateOptions{})
	require.NoError(t, err)
}
