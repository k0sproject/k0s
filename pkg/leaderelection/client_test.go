/*
Copyright 2024 k0s authors

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
	"sync"
	"sync/atomic"
	"testing"
	"time"

	coordinationv1 "k8s.io/api/coordination/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
	fakecoordinationv1 "k8s.io/client-go/kubernetes/typed/coordination/v1/fake"
	k8stesting "k8s.io/client-go/testing"
	"k8s.io/utils/ptr"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLeaseConfig_Name(t *testing.T) {
	client, err := NewClient(&LeaseConfig{
		Namespace: "foo",
		Identity:  "bar",
		Client:    fake.NewSimpleClientset().CoordinationV1(),
	})
	assert.Nil(t, client)
	assert.ErrorContains(t, err, "name may not be empty")
}

func TestLeaseConfig_Identity(t *testing.T) {
	client, err := NewClient(&LeaseConfig{
		Namespace: "foo",
		Name:      "bar",
		Client:    fake.NewSimpleClientset().CoordinationV1(),
	})
	assert.Nil(t, client)
	assert.ErrorContains(t, err, "Lock identity is empty")
}

func TestLeaseConfig_Client(t *testing.T) {
	client, err := NewClient(&LeaseConfig{
		Namespace: "foo",
		Name:      "bar",
		Identity:  "baz",
	})
	assert.Nil(t, client)
	assert.ErrorContains(t, err, "client may not be nil")
}

func TestClient_Reacquisition(t *testing.T) {
	fakeClient := fake.NewSimpleClientset()
	ctx, cancel := context.WithCancel(context.TODO())
	t.Cleanup(cancel)

	givenLeaderElectorError := func() func(err error) {
		var updateErr atomic.Pointer[error]
		updateErr.Store(ptr.To[error](nil))
		fakeClient.CoordinationV1().(*fakecoordinationv1.FakeCoordinationV1).PrependReactor("update", "leases", func(action k8stesting.Action) (bool, runtime.Object, error) {
			if errPtr := updateErr.Load(); *errPtr != nil {
				return true, nil, *errPtr
			}
			return false, nil, nil
		})

		return func(err error) {
			updateErr.Store(&err)
		}
	}()

	observedCallbacks := 0
	ct := &cancellingT{t, cancel}
	callbacks := []func(Status){
		func(status Status) {
			assert.Equal(ct, StatusLeading, status, "Should take the lead when run")
			if t.Failed() {
				return
			}
			t.Log("Took the lead, disrupting leader election and waiting to loose the lead")
			givenLeaderElectorError(errors.New("leader election disrupted by test case"))
		},
		func(status Status) {
			assert.Equal(ct, StatusPending, status, "Should loose the lead when disrupted")
			if t.Failed() {
				return
			}
			t.Log("Lost the lead, restoring the leader election, and waiting to regain the lead")
			givenLeaderElectorError(nil)
		},
		func(status Status) {
			assert.Equal(ct, StatusLeading, status, "Should regain the lead")
			if t.Failed() {
				return
			}
			t.Log("Regained the lead after leader election was restored")
			cancel()
		},
		func(status Status) {
			assert.Equal(ct, StatusPending, status, "Should drop the lead when context is done")
		},
	}

	underTest, err := NewClient(&LeaseConfig{
		Namespace: "foo", Name: "bar", Identity: t.Name(),
		Client: fakeClient.CoordinationV1(),
		internalConfig: internalConfig{
			renewDeadline: 10 * time.Millisecond,
			retryPeriod:   5 * time.Millisecond,
		},
	})
	require.NoError(t, err)

	underTest.Run(ctx, func(status Status) {
		assert.Less(ct, observedCallbacks, len(callbacks), "Callback called too often")
		if t.Failed() {
			return
		}
		callbacks[observedCallbacks](status)
		observedCallbacks++
	})

	assert.Equal(t, len(callbacks), observedCallbacks, "Callback not called often enough")
}

func TestClient_LeadTakeover(t *testing.T) {
	fakeClient := fake.NewSimpleClientset()
	ctx, cancel := context.WithCancel(context.TODO())
	t.Cleanup(cancel)

	// Create two leader election clients, Red and Black.
	ctxRed, cancelRed := context.WithCancel(ctx)
	red, err := NewClient(&LeaseConfig{
		Namespace: "foo", Name: "bar", Identity: "Red",
		Client: fakeClient.CoordinationV1(),
		internalConfig: internalConfig{
			renewDeadline: 10 * time.Millisecond,
			retryPeriod:   5 * time.Millisecond,
		},
	})
	require.NoError(t, err)
	ctxBlack, cancelBlack := context.WithCancel(ctx)
	black, err := NewClient(&LeaseConfig{
		Namespace: "foo", Name: "bar", Identity: "Black",
		Client: fakeClient.CoordinationV1(),
		internalConfig: internalConfig{
			renewDeadline: 10 * time.Millisecond,
			retryPeriod:   5 * time.Millisecond,
		},
	})
	require.NoError(t, err)

	// Let Red and Black run concurrently. Red will take the lead first.
	// Cancel Red's context and ensure that Black takes over the lead.
	// Finally, cancel Black's context, so that the test terminates.

	var observedCallbacks atomic.Uint32
	ct := &cancellingT{t, cancel}
	callbacks := []func(string, Status){
		func(runner string, status Status) {
			assert.Equal(ct, "Red", runner, "Red should take the lead first")
			assert.Equal(ct, StatusLeading, status, "Red should take the lead first")
			if t.Failed() {
				return
			}
			t.Log("Red took the lead, cancelling Red's context")
			cancelRed()
		},
		func(runner string, status Status) {
			assert.Equal(ct, "Red", runner, "Red should drop the lead first")
			assert.Equal(ct, StatusPending, status, "Red should drop the lead first")
			if t.Failed() {
				return
			}
			t.Log("Red dropped the lead")
		},
		func(runner string, status Status) {
			assert.Equal(ct, "Black", runner, "Black should take the lead after Red")
			assert.Equal(ct, StatusLeading, status, "Black should take the lead after Red")
			if t.Failed() {
				return
			}
			t.Log("Black took the lead, cancelling Black's context")
			cancelBlack()
		},
		func(runner string, status Status) {
			assert.Equal(ct, "Black", runner, "Black should finally drop the lead")
			assert.Equal(ct, StatusPending, status, "Black should finally drop the lead")
			if t.Failed() {
				return
			}
			t.Log("Black dropped the lead")
		},
	}

	// Pre-create the acquired lease for Red, so that there are no races when
	// taking the lead by the two competing leader election client.
	now := metav1.NewMicroTime(time.Now())
	_, err = fakeClient.CoordinationV1().Leases("foo").Create(context.TODO(), &coordinationv1.Lease{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Lease",
			APIVersion: coordinationv1.SchemeGroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: "bar",
		},
		Spec: coordinationv1.LeaseSpec{
			HolderIdentity:       ptr.To("Red"),
			AcquireTime:          &now,
			RenewTime:            &now,
			LeaseDurationSeconds: ptr.To(int32((1 * time.Hour).Seconds())), // block lease for a very long time
		},
	}, metav1.CreateOptions{})
	require.NoError(t, err)
	t.Log("Pre-created acquired lease for Red")

	// Run the two clients concurrently.
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		red.Run(ctxRed, func(status Status) {
			offset := observedCallbacks.Add(1) - 1
			assert.Less(ct, int(offset), len(callbacks), "Callback called too often")
			if t.Failed() {
				return
			}
			callbacks[offset]("Red", status)
		})
	}()
	go func() {
		defer wg.Done()
		black.Run(ctxBlack, func(status Status) {
			offset := observedCallbacks.Add(1) - 1
			assert.Less(ct, int(offset), len(callbacks), "Callback called too often")
			if t.Failed() {
				return
			}
			callbacks[offset]("Black", status)
		})
	}()
	wg.Wait()

	assert.Equal(t, uint32(len(callbacks)), observedCallbacks.Load(), "Callback not called often enough")
}

// Small helper that cancels a context as soon as an error is reported.
type cancellingT struct {
	delegate assert.TestingT
	cancel   context.CancelFunc
}

func (t *cancellingT) Errorf(format string, args ...any) {
	t.delegate.Errorf(format, args...)
	t.cancel()
}
