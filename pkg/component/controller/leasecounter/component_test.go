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
	Lease  = leasecounter.Lease
	Leases = leasecounter.Leases
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
	} {
		t.Run(tt.name, func(t *testing.T) {
			leases := runOneRound(t, tt.leases...)

			assert.Equal(t, tt.expectedCount, leases.ActiveCount)
			assert.Equal(t, tt.expectedOwn, leases.Own)
			assert.ElementsMatch(t, tt.expectedAll, leases.All)
		})
	}
}

// Runs a single lease counter round and returns what the round handed over.
func runOneRound(t *testing.T, objects ...runtime.Object) Leases {
	t.Helper()
	var observedLeases []Leases

	now := time.Now()
	synctest.Test(t, func(t *testing.T) {
		// Bring the bubble's clock in line, so that expiry times align.
		time.Sleep(time.Until(now))

		factory := testutil.NewFakeClientFactory(objects...)
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
			UpdateLeases:      func(l Leases) { observedLeases = append(observedLeases, l) },
		}

		require.NoError(t, underTest.Init(t.Context()))
		require.NoError(t, underTest.Start(t.Context()))
		synctest.Wait()
		require.NoError(t, underTest.Stop())
	})

	require.Len(t, observedLeases, 1, "Expected a single lease update")
	return observedLeases[0]
}
