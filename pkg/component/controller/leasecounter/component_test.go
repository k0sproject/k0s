// SPDX-FileCopyrightText: 2026 k0s authors
// SPDX-License-Identifier: Apache-2.0

package leasecounter_test

import (
	"errors"
	"testing"
	"testing/synctest"
	"time"

	"github.com/k0sproject/k0s/pkg/component/controller/leasecounter"

	coordinationv1 "k8s.io/api/coordination/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"

	"github.com/k0sproject/k0s/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	clientgotesting "k8s.io/client-go/testing"
)

const (
	ownNodeName  = "self"
	ownLeaseName = "k0s-ctrl-" + ownNodeName
)

type (
	KonnectivityState = leasecounter.KonnectivityState
	Lease             = leasecounter.Lease
	Leases            = leasecounter.Leases
)

func TestComponent_LeaseCounter(t *testing.T) {
	for _, tt := range []struct {
		name          string
		leases        []runtime.Object
		expectedCount uint
		expectedOwn   *Lease
		expectedAll   []Lease
	}{
		{
			"counts itself while its own lease doesn't exist yet",
			[]runtime.Object{makeLease(withName("k0s-ctrl-other"))},
			2, nil, []Lease{{Name: "k0s-ctrl-other", Active: true}},
		},
		{
			"doesn't count itself twice while its own lease is active",
			[]runtime.Object{makeLease(withName("k0s-ctrl-other")), makeLease(withName(ownLeaseName))},
			2, &Lease{Name: ownLeaseName, Active: true},
			[]Lease{{Name: "k0s-ctrl-other", Active: true}, {Name: ownLeaseName, Active: true}},
		},
		{
			"counts itself when its own lease has expired",
			[]runtime.Object{makeLease(withName("k0s-ctrl-other")), makeLease(withName(ownLeaseName), expired)},
			2, &Lease{Name: ownLeaseName, Active: false},
			[]Lease{{Name: "k0s-ctrl-other", Active: true}, {Name: ownLeaseName, Active: false}},
		},
		{
			"hands over expired leases without counting them",
			[]runtime.Object{makeLease(withName("k0s-ctrl-other"), expired), makeLease(withName(ownLeaseName))},
			1, &Lease{Name: ownLeaseName, Active: true},
			[]Lease{{Name: "k0s-ctrl-other", Active: false}, {Name: ownLeaseName, Active: true}},
		},
		{
			"hands over released leases but not non-controller leases",
			[]runtime.Object{
				makeLease(withName(ownLeaseName)),
				makeLease(withoutControllerLabel, withName("some-other-lease")),
				makeLease(withName("k0s-ctrl-released"), func(l *coordinationv1.Lease) { l.Spec.HolderIdentity = new("") }),
			},
			1, &Lease{Name: ownLeaseName, Active: true},
			[]Lease{{Name: ownLeaseName, Active: true}, {Name: "k0s-ctrl-released", Active: false}},
		},
		{
			"parses the recorded konnectivity state",
			[]runtime.Object{
				makeLease(withName(ownLeaseName), withAnnotations(map[string]string{
					"k0s.k0sproject.io/konnectivity-servers-state": "3/4",
				})),
				makeLease(withName("k0s-ctrl-other")),
			},
			2, &Lease{Name: ownLeaseName, Active: true, Konnectivity: &KonnectivityState{ServerCount: 3, ServerCountRises: 4}},
			[]Lease{
				{Name: ownLeaseName, Active: true, Konnectivity: &KonnectivityState{ServerCount: 3, ServerCountRises: 4}},
				{Name: "k0s-ctrl-other", Active: true},
			},
		},
		{
			"treats unparsable konnectivity state values as zero",
			[]runtime.Object{
				makeLease(withName(ownLeaseName), withAnnotations(map[string]string{
					"k0s.k0sproject.io/konnectivity-servers-state": "three/4",
				})),
			},
			1, &Lease{Name: ownLeaseName, Active: true},
			[]Lease{{Name: ownLeaseName, Active: true}},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			leases, _, _ := runOneRound(t, nil, tt.leases...)

			assert.Equal(t, tt.expectedCount, leases.ActiveCount)
			assert.Equal(t, tt.expectedOwn, leases.Own)
			assert.ElementsMatch(t, tt.expectedAll, leases.All)
		})
	}
}

func TestComponent_KonnectivityState(t *testing.T) {
	patches := func(actions []clientgotesting.Action) (patches uint) {
		for _, action := range actions {
			if action.GetVerb() == "patch" {
				patches++
			}
		}
		return
	}

	t.Run("records the state returned by the update", func(t *testing.T) {
		_, lease, _ := runOneRound(t,
			&KonnectivityState{ServerCount: 3, ServerCountRises: 4},
			makeLease(withName(ownLeaseName), withAnnotations(map[string]string{"example.com/keep": "me"})),
		)

		require.NotNil(t, lease, "Expected the controller lease to be present")
		assert.Equal(t,
			map[string]string{
				"example.com/keep": "me",
				"k0s.k0sproject.io/konnectivity-servers-state": "3/4",
			},
			lease.Annotations,
			"Expected the state to be recorded without touching other annotations",
		)
	})

	t.Run("doesn't patch when the state is already recorded", func(t *testing.T) {
		_, _, actions := runOneRound(t,
			&KonnectivityState{ServerCount: 3, ServerCountRises: 4},
			makeLease(withName(ownLeaseName), withAnnotations(map[string]string{
				"k0s.k0sproject.io/konnectivity-servers-state": "3/4",
			})),
		)

		assert.Zero(t, patches(actions), "Expected no patch to be sent")
	})

	t.Run("overwrites an outdated state", func(t *testing.T) {
		_, lease, _ := runOneRound(t,
			&KonnectivityState{ServerCount: 3, ServerCountRises: 5},
			makeLease(withName(ownLeaseName), withAnnotations(map[string]string{
				"k0s.k0sproject.io/konnectivity-servers-state": "2/4",
			})),
		)

		require.NotNil(t, lease, "Expected the controller lease to be present")
		assert.Equal(t,
			map[string]string{"k0s.k0sproject.io/konnectivity-servers-state": "3/5"},
			lease.Annotations,
		)
	})

	t.Run("waits for its own lease to exist", func(t *testing.T) {
		leases, _, actions := runOneRound(t,
			&KonnectivityState{ServerCount: 3, ServerCountRises: 4},
			makeLease(withName("k0s-ctrl-other")),
		)

		assert.Nil(t, leases.Own)
		assert.Zero(t, patches(actions), "Expected no patch to be sent for a non-existent lease")
	})

	t.Run("clears a recorded state if the update returns none", func(t *testing.T) {
		_, lease, _ := runOneRound(t,
			nil,
			makeLease(withName(ownLeaseName), withAnnotations(map[string]string{
				"example.com/keep": "me", "k0s.k0sproject.io/konnectivity-servers-state": "3/4",
			})),
		)

		require.NotNil(t, lease, "Expected the controller lease to be present")
		assert.Equal(t, map[string]string{"example.com/keep": "me"}, lease.Annotations,
			"Expected the state to be removed without touching other annotations",
		)
	})

	t.Run("doesn't patch when there's nothing to clear", func(t *testing.T) {
		_, _, actions := runOneRound(t,
			nil,
			makeLease(withName(ownLeaseName)),
		)

		assert.Zero(t, patches(actions), "Expected no patch to be sent")
	})
}

// Runs a single lease counter round, having the update return the given
// konnectivity state, and returns what the round handed over.
func runOneRound(t *testing.T, state *KonnectivityState, objects ...runtime.Object) (Leases, *coordinationv1.Lease, []clientgotesting.Action) {
	t.Helper()

	var (
		factory        *testutil.FakeClientFactory
		observedLeases []Leases
		ownLease       *coordinationv1.Lease
	)

	now := time.Now()
	synctest.Test(t, func(t *testing.T) {
		// Bring the bubble's clock in line, so that expiry times align.
		time.Sleep(time.Until(now))

		factory = testutil.NewFakeClientFactory(objects...)
		factory.Client.PrependReactor("*", "leases", func(a clientgotesting.Action) (bool, runtime.Object, error) {
			switch a.GetVerb() {
			case "create", "update":
				return true, nil, errors.New("leader election writes are disabled in this test")
			}
			return false, nil, nil
		})

		underTest := leasecounter.Component{
			NodeName:          types.NodeName(ownNodeName),
			InvocationID:      t.Name(),
			KubeClientFactory: factory,
			UpdateLeases: func(l Leases) *KonnectivityState {
				observedLeases = append(observedLeases, l)
				return state
			},
		}

		require.NoError(t, underTest.Init(t.Context()))
		require.NoError(t, underTest.Start(t.Context()))
		t.Cleanup(func() { assert.NoError(t, underTest.Stop()) })
		synctest.Wait()

		var err error
		ownLease, err = factory.Client.CoordinationV1().Leases(corev1.NamespaceNodeLease).Get(t.Context(), ownLeaseName, metav1.GetOptions{})
		if !apierrors.IsNotFound(err) {
			require.NoError(t, err)
		}
	})

	require.Len(t, observedLeases, 1, "Expected a single lease update")
	return observedLeases[0], ownLease, factory.Client.Actions()
}

func withAnnotations(annotations map[string]string) func(*coordinationv1.Lease) {
	return func(l *coordinationv1.Lease) {
		l.Annotations = annotations
	}
}
