package leaderelection

import (
	"context"
	"fmt"
	"github.com/stretchr/testify/assert"
	coordinationv1 "k8s.io/api/coordination/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
	"testing"
)

func TestLeasePoolWatcherTriggersOnLeaseAcquisition(t *testing.T) {
	fakeClient := fake.NewSimpleClientset()
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
	assert.NoError(t, err)

	identity := "test-node"
	_, err = fakeClient.CoordinationV1().Leases("test").Create(context.TODO(), &coordinationv1.Lease{
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

	assert.NoError(t, err)

	p, err := NewLeasePool(fakeClient, "test", WithIdentity(identity), WithNamespace("test"))
	assert.NoError(t, err)

	output := &LeaseEvents{
		AcquiredLease: make(chan struct{}, 1),
		LostLease:     make(chan struct{}, 1),
	}

	events, cancel, err := p.Watch(WithOutputChannels(output))
	assert.NoError(t, err)

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
		fmt.Println("successfully acquired lease")
	case <-failed:
		t.Error("lost lease")
	}

	cancel()
}

func TestLeasePoolTriggersLostLeaseWhenCancelled(t *testing.T) {
	fakeClient := fake.NewSimpleClientset()
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
	assert.NoError(t, err)

	identity := "test-node"
	_, err = fakeClient.CoordinationV1().Leases("test").Create(context.TODO(), &coordinationv1.Lease{
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

	assert.NoError(t, err)

	p, err := NewLeasePool(fakeClient, "test", WithIdentity(identity), WithNamespace("test"))
	assert.NoError(t, err)

	output := &LeaseEvents{
		AcquiredLease: make(chan struct{}, 1),
		LostLease:     make(chan struct{}, 1),
	}

	events, cancel, err := p.Watch(WithOutputChannels(output))
	assert.NoError(t, err)

	<-events.AcquiredLease
	fmt.Println("lease acquired, cancelling leaser")
	cancel()
	<-events.LostLease
	fmt.Println("context cancelled and lease successfully lost")
}

func TestSecondWatcherAcquiresReleasedLease(t *testing.T) {
	fakeClient := fake.NewSimpleClientset()
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
	assert.NoError(t, err)

	identity := "test-node"
	leaseDurationSeconds := int32(1)
	_, err = fakeClient.CoordinationV1().Leases("test").Create(context.TODO(), &coordinationv1.Lease{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Lease",
			APIVersion: "coordination.k8s.io/v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: "test",
		},
		Spec: coordinationv1.LeaseSpec{
			HolderIdentity: &identity,
			LeaseDurationSeconds: &leaseDurationSeconds,
		},
	}, metav1.CreateOptions{})

	assert.NoError(t, err)

	p, err := NewLeasePool(fakeClient, "test", WithIdentity(identity), WithNamespace("test"))
	assert.NoError(t, err)

	events, cancel, err := p.Watch(WithOutputChannels(&LeaseEvents{
		AcquiredLease: make(chan struct{}, 1),
		LostLease:     make(chan struct{}, 1),
	}))

	assert.NoError(t, err)
	identity2 := "test-node-2"
	p2, err := NewLeasePool(fakeClient, "test", WithIdentity(identity2), WithNamespace("test"))
	assert.NoError(t, err)

	events2, cancel2, err := p2.Watch(WithOutputChannels(&LeaseEvents{
		AcquiredLease: make(chan struct{}, 1),
		LostLease:     make(chan struct{}, 1),
	}))
	assert.NoError(t, err)
	fmt.Println("started second lease holder")
	defer cancel2()

	var receivedEvents []string

leaseEventLoop:
	for {
		select {
		case <-events.AcquiredLease:
			fmt.Println("lease acquired, cancelling leaser")
			cancel()
			receivedEvents = append(receivedEvents, "node1-acquired")
		case <-events.LostLease:
			fmt.Println("context cancelled and node 1 lease successfully lost")
			receivedEvents = append(receivedEvents, "node1-lost")
		case <-events2.AcquiredLease:
			fmt.Println("node 2 lease acquired")
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
