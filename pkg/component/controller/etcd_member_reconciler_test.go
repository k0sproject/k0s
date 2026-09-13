// SPDX-FileCopyrightText: 2026 k0s authors
// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"testing"

	etcdv1beta1 "github.com/k0sproject/k0s/pkg/apis/etcd/v1beta1"
	"github.com/k0sproject/k0s/pkg/apis/k0s/v1beta1"
	k0sfake "github.com/k0sproject/k0s/pkg/client/clientset/fake"
	"github.com/k0sproject/k0s/pkg/etcd"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8stesting "k8s.io/client-go/testing"
)

func assertStatusSubresourceUpdated(t *testing.T, actions []k8stesting.Action) {
	t.Helper()
	for _, action := range actions {
		if update, ok := action.(k8stesting.UpdateAction); ok && update.GetSubresource() == "status" {
			return
		}
	}
	t.Error("expected an update to the status subresource")
}

func TestReconcileMember(t *testing.T) {
	reconcile := func(t *testing.T, e *EtcdMemberReconciler, member *etcdv1beta1.EtcdMember, clusterMembers []etcd.Member) (bool, *etcdv1beta1.EtcdMember, []k8stesting.Action) {
		t.Helper()
		clientset := k0sfake.NewSimpleClientset(member)
		client := clientset.EtcdV1beta1().EtcdMembers()

		got := e.reconcileMember(t.Context(), nil, client, clusterMembers, member)
		actions := clientset.Actions()

		updated, err := client.Get(t.Context(), member.Name, metav1.GetOptions{})
		require.NoError(t, err, "expected to be able to read the member back from the fake client")

		return got, updated, actions
	}

	t.Run("marks member as failed when member ID is invalid", func(t *testing.T) {
		e := &EtcdMemberReconciler{}
		member := &etcdv1beta1.EtcdMember{
			ObjectMeta: metav1.ObjectMeta{Name: "member-1"},
			Status:     etcdv1beta1.Status{MemberID: "not-hex"},
		}

		got, updated, actions := reconcile(t, e, member, nil)

		assert.False(t, got, "reconcileMember should report failure for an invalid member ID")
		assert.Equal(t, etcdv1beta1.ReconcileStatusFailed, updated.Status.ReconcileStatus)
		assertStatusSubresourceUpdated(t, actions)
	})

	t.Run("does nothing when member is already recorded as not joined", func(t *testing.T) {
		e := &EtcdMemberReconciler{}
		member := &etcdv1beta1.EtcdMember{
			ObjectMeta: metav1.ObjectMeta{Name: "member-1"},
			Status: etcdv1beta1.Status{
				MemberID:   "1",
				Conditions: []etcdv1beta1.JoinCondition{{Type: etcdv1beta1.ConditionTypeJoined, Status: etcdv1beta1.ConditionFalse}},
			},
		}

		got, _, actions := reconcile(t, e, member, nil)

		assert.True(t, got)
		assert.Empty(t, actions, "a member already recorded as not joined shouldn't be touched again")
	})

	t.Run("records member as left when join status was never set", func(t *testing.T) {
		e := &EtcdMemberReconciler{}
		member := &etcdv1beta1.EtcdMember{
			ObjectMeta: metav1.ObjectMeta{Name: "member-1"},
			Status:     etcdv1beta1.Status{MemberID: "1"},
		}

		got, updated, actions := reconcile(t, e, member, nil)

		assert.True(t, got)
		assert.Equal(t, etcdv1beta1.ReconcileStatusSuccess, updated.Status.ReconcileStatus)
		cond := updated.Status.GetCondition(etcdv1beta1.ConditionTypeJoined)
		if assert.NotNil(t, cond, "expected a Joined condition to be set") {
			assert.Equal(t, etcdv1beta1.ConditionFalse, cond.Status)
		}
		assertStatusSubresourceUpdated(t, actions)
	})

	t.Run("records member as left when previously joined", func(t *testing.T) {
		e := &EtcdMemberReconciler{}
		member := &etcdv1beta1.EtcdMember{
			ObjectMeta: metav1.ObjectMeta{Name: "member-1"},
			Status: etcdv1beta1.Status{
				MemberID:   "1",
				Conditions: []etcdv1beta1.JoinCondition{{Type: etcdv1beta1.ConditionTypeJoined, Status: etcdv1beta1.ConditionTrue}},
			},
		}

		got, updated, actions := reconcile(t, e, member, nil)

		assert.True(t, got)
		assert.Equal(t, etcdv1beta1.ReconcileStatusSuccess, updated.Status.ReconcileStatus)
		cond := updated.Status.GetCondition(etcdv1beta1.ConditionTypeJoined)
		if assert.NotNil(t, cond, "expected the Joined condition to be updated") {
			assert.Equal(t, etcdv1beta1.ConditionFalse, cond.Status)
		}
		assertStatusSubresourceUpdated(t, actions)
	})

	t.Run("does nothing when member is not marked to leave", func(t *testing.T) {
		e := &EtcdMemberReconciler{}
		member := &etcdv1beta1.EtcdMember{
			ObjectMeta: metav1.ObjectMeta{Name: "member-1"},
			Status:     etcdv1beta1.Status{MemberID: "1"},
		}
		clusterMembers := []etcd.Member{{ID: 1}}

		got, _, actions := reconcile(t, e, member, clusterMembers)

		assert.True(t, got)
		assert.Empty(t, actions, "a member that isn't marked to leave shouldn't be touched")
	})

	t.Run("refuses to leave when it is the last controller", func(t *testing.T) {
		e := &EtcdMemberReconciler{
			etcdConfig:      &v1beta1.EtcdConfig{PeerAddress: "self"},
			controllerCount: func() uint { return 1 },
		}
		member := &etcdv1beta1.EtcdMember{
			ObjectMeta: metav1.ObjectMeta{Name: "member-1"},
			Status:     etcdv1beta1.Status{MemberID: "1", PeerAddress: "self"},
			Spec:       etcdv1beta1.EtcdMemberSpec{Leave: true},
		}
		clusterMembers := []etcd.Member{{ID: 1}}

		got, updated, actions := reconcile(t, e, member, clusterMembers)

		assert.False(t, got, "reconcileMember should refuse to let the last controller leave")
		assert.Empty(t, updated.Status.ReconcileStatus)
		assert.Contains(t, updated.Status.Message, "only active controller")
		assertStatusSubresourceUpdated(t, actions)
	})
}
