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
	k8stesting "k8s.io/client-go/testing"
)

func TestReconcileMember(t *testing.T) {
	reconcile := func(t *testing.T, member *etcdv1beta1.EtcdMember, clusterMembers []etcd.Member, peerAddress string, controllerCount uint) (bool, []k8stesting.Action) {
		t.Helper()
		clientset := k0sfake.NewSimpleClientset(member)
		client := clientset.EtcdV1beta1().EtcdMembers()
		e := &EtcdMemberReconciler{
			etcdConfig:      &v1beta1.EtcdConfig{PeerAddress: peerAddress},
			controllerCount: func() uint { return controllerCount },
		}

		got := e.reconcileMember(t.Context(), nil, client, clusterMembers, member)
		return got, clientset.Actions()
	}

	t.Run("marks member as failed when member ID is invalid", func(t *testing.T) {
		member := &etcdv1beta1.EtcdMember{Status: etcdv1beta1.Status{MemberID: "not-hex"}}

		got, _ := reconcile(t, member, nil, "", 0)

		assert.False(t, got, "reconcileMember should report failure for an invalid member ID")
		assert.Equal(t, etcdv1beta1.ReconcileStatusFailed, member.Status.ReconcileStatus)
	})

	t.Run("does nothing when member is already recorded as not joined", func(t *testing.T) {
		member := &etcdv1beta1.EtcdMember{Status: etcdv1beta1.Status{
			MemberID:   "1",
			Conditions: []etcdv1beta1.JoinCondition{{Type: etcdv1beta1.ConditionTypeJoined, Status: etcdv1beta1.ConditionFalse}},
		}}

		got, actions := reconcile(t, member, nil, "", 0)

		assert.True(t, got)
		assert.Empty(t, actions, "a member already recorded as not joined shouldn't be touched again")
	})

	t.Run("records member as left when join status was never set", func(t *testing.T) {
		member := &etcdv1beta1.EtcdMember{Status: etcdv1beta1.Status{MemberID: "1"}}

		got, _ := reconcile(t, member, nil, "", 0)

		assert.True(t, got)
		assert.Equal(t, etcdv1beta1.ReconcileStatusSuccess, member.Status.ReconcileStatus)
		cond := member.Status.GetCondition(etcdv1beta1.ConditionTypeJoined)
		if assert.NotNil(t, cond, "expected a Joined condition to be set") {
			assert.Equal(t, etcdv1beta1.ConditionFalse, cond.Status)
		}
	})

	t.Run("records member as left when previously joined", func(t *testing.T) {
		member := &etcdv1beta1.EtcdMember{Status: etcdv1beta1.Status{
			MemberID:   "1",
			Conditions: []etcdv1beta1.JoinCondition{{Type: etcdv1beta1.ConditionTypeJoined, Status: etcdv1beta1.ConditionTrue}},
		}}

		got, _ := reconcile(t, member, nil, "", 0)

		assert.True(t, got)
		assert.Equal(t, etcdv1beta1.ReconcileStatusSuccess, member.Status.ReconcileStatus)
		cond := member.Status.GetCondition(etcdv1beta1.ConditionTypeJoined)
		if assert.NotNil(t, cond, "expected the Joined condition to be updated") {
			assert.Equal(t, etcdv1beta1.ConditionFalse, cond.Status)
		}
	})

	t.Run("does nothing when member is not marked to leave", func(t *testing.T) {
		member := &etcdv1beta1.EtcdMember{Status: etcdv1beta1.Status{MemberID: "1"}}
		clusterMembers := []etcd.Member{{ID: 1}}

		got, actions := reconcile(t, member, clusterMembers, "", 0)

		assert.True(t, got)
		assert.Empty(t, actions, "a member that isn't marked to leave shouldn't be touched")
	})

	t.Run("refuses to leave when it is the last controller", func(t *testing.T) {
		member := &etcdv1beta1.EtcdMember{
			Status: etcdv1beta1.Status{MemberID: "1", PeerAddress: "self"},
			Spec:   etcdv1beta1.EtcdMemberSpec{Leave: true},
		}
		clusterMembers := []etcd.Member{{ID: 1}}

		got, _ := reconcile(t, member, clusterMembers, "self", 1)

		assert.False(t, got, "reconcileMember should refuse to let the last controller leave")
		assert.Empty(t, member.Status.ReconcileStatus)
		assert.Contains(t, member.Status.Message, "only active controller")
	})
}
