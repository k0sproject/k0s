// SPDX-FileCopyrightText: 2026 k0s authors
// SPDX-License-Identifier: Apache-2.0

package leasecounter

import (
	"testing"
	"time"

	coordinationv1 "k8s.io/api/coordination/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/stretchr/testify/assert"
)

func TestActiveLeases(t *testing.T) {
	noHolderIdentity := func(l *coordinationv1.Lease) { l.Spec.HolderIdentity = nil }
	emptyHolderIdentity := func(l *coordinationv1.Lease) { l.Spec.HolderIdentity = new("") }

	for _, tt := range []struct {
		name     string
		expected uint
		leases   []runtime.Object
	}{
		{
			"counts a single active controller lease",
			1, []runtime.Object{makeLease()},
		},
		{
			"returns zero when there are no leases",
			0, nil,
		},
		{
			"ignores leases without the controller prefix",
			0, []runtime.Object{makeLease(withName("some-other-lease"))},
		},
		{
			"ignores leases with no holder identity",
			0, []runtime.Object{makeLease(noHolderIdentity)},
		},
		{
			"ignores leases with an empty holder identity",
			0, []runtime.Object{makeLease(emptyHolderIdentity)},
		},
		{
			"ignores expired leases",
			0, []runtime.Object{makeLease(expired)},
		},
		{
			"counts only the active controller leases among a mix",
			3, []runtime.Object{
				makeLease(withName("k0s-ctrl-0")),
				makeLease(withName("k0s-ctrl-1")),
				makeLease(withName("k0s-ctrl-2")),
				// Not counted: doesn't have the controller prefix.
				makeLease(withName("kube-node-lease-holder")),
				// Not counted: no holder identity.
				makeLease(withName("k0s-ctrl-no-holder"), noHolderIdentity),
				// Not counted: expired.
				makeLease(withName("k0s-ctrl-expired"), expired),
			},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			c := fake.NewSimpleClientset(tt.leases...)
			count, err := ActiveLeases(t.Context(), c)
			assert.NoError(t, err)
			assert.Equal(t, tt.expected, count)
		})
	}
}

func makeLease(funcs ...func(*coordinationv1.Lease)) *coordinationv1.Lease {
	now := metav1.NowMicro()
	lease := &coordinationv1.Lease{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "k0s-ctrl-foo",
			Namespace: corev1.NamespaceNodeLease,
		},
		Spec: coordinationv1.LeaseSpec{
			HolderIdentity:       new("foo"),
			LeaseDurationSeconds: new(int32(60)),
			AcquireTime:          &now,
			RenewTime:            &now,
		},
	}
	for _, f := range funcs {
		f(lease)
	}
	return lease
}

func withName(name string) func(*coordinationv1.Lease) {
	return func(l *coordinationv1.Lease) {
		l.Name = name
	}
}

// Pushes the lease's renew time far enough into the past that its duration has
// elapsed.
func expired(l *coordinationv1.Lease) {
	dur := time.Duration(*l.Spec.LeaseDurationSeconds) * time.Second
	renewed := metav1.NewMicroTime(time.Now().Add(-dur - time.Second))
	l.Spec.RenewTime = &renewed
}
