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
			"counts a controller lease identified by label",
			1, []runtime.Object{makeLease()},
		},
		{
			"counts a legacy controller lease identified by name prefix",
			1, []runtime.Object{makeLease(withoutControllerLabel, withName("k0s-ctrl-foo"))},
		},
		{
			"returns zero when there are no leases",
			0, nil,
		},
		{
			"ignores leases without the controller label or name prefix",
			0, []runtime.Object{makeLease(withoutControllerLabel, withName("some-other-lease"))},
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
				// Counted: identified by label.
				makeLease(withName("lease-0")),
				makeLease(withName("lease-1")),
				// FIXME: remove in k0s 1.38+
				// Counted: legacy lease identified by name prefix.
				makeLease(withoutControllerLabel, withName("k0s-ctrl-legacy")),
				// Not counted: neither label nor prefix.
				makeLease(withoutControllerLabel, withName("kube-node-lease-holder")),
				// Not counted: no holder identity.
				makeLease(withName("lease-no-holder"), noHolderIdentity),
				// Not counted: expired.
				makeLease(withName("lease-expired"), expired),
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
			Name:      "foo",
			Namespace: corev1.NamespaceNodeLease,
			Labels: map[string]string{
				leaseTypeLabel: leaseTypeLabelValueController,
			},
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

// Removes the controller lease type label, so the lease is only considered a
// controller lease if its name carries the legacy prefix.
func withoutControllerLabel(l *coordinationv1.Lease) {
	delete(l.Labels, leaseTypeLabel)
}

// Pushes the lease's renew time far enough into the past that its duration has
// elapsed.
func expired(l *coordinationv1.Lease) {
	dur := time.Duration(*l.Spec.LeaseDurationSeconds) * time.Second
	renewed := metav1.NewMicroTime(time.Now().Add(-dur - time.Second))
	l.Spec.RenewTime = &renewed
}
