// SPDX-FileCopyrightText: 2026 k0s authors
// SPDX-License-Identifier: Apache-2.0

package api

import (
	"testing"

	etcdv1beta1 "github.com/k0sproject/k0s/pkg/apis/etcd/v1beta1"
	k0sfake "github.com/k0sproject/k0s/pkg/client/clientset/fake"
	"github.com/k0sproject/k0s/pkg/etcd"

	coordinationv1 "k8s.io/api/coordination/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	kubefake "k8s.io/client-go/kubernetes/fake"
	"k8s.io/utils/ptr"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLeaderPromotesLearners(t *testing.T) {
	lease := func(name, holder string) runtime.Object {
		return &coordinationv1.Lease{
			ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: corev1.NamespaceNodeLease},
			Spec:       coordinationv1.LeaseSpec{HolderIdentity: ptr.To(holder)},
		}
	}
	member := func(name string, labels map[string]string) runtime.Object {
		return &etcdv1beta1.EtcdMember{ObjectMeta: metav1.ObjectMeta{Name: name, Labels: labels}}
	}
	// controller0 leads under the invocation ID "leader".
	leases := []runtime.Object{
		lease(leaderLeaseName, "leader"),
		lease("k0s-ctrl-controller0", "leader"),
		lease("k0s-ctrl-controller1", "other"),
	}
	otherMember := member("controller1", map[string]string{
		etcd.ControllerLeaseLabel:  "k0s-ctrl-controller1",
		etcd.PromotesLearnersLabel: "other",
	})

	for _, test := range []struct {
		name     string
		leases   []runtime.Object
		members  []runtime.Object
		expected bool
		err      string
	}{
		{
			name: "leader advertises promotion",
			members: []runtime.Object{otherMember, member("controller0", map[string]string{
				etcd.ControllerLeaseLabel:  "k0s-ctrl-controller0",
				etcd.PromotesLearnersLabel: "leader",
			})},
			expected: true,
		},
		{
			name: "leader found by its controller lease label",
			members: []runtime.Object{otherMember, member("etcd-name-differs", map[string]string{
				etcd.ControllerLeaseLabel:  "k0s-ctrl-controller0",
				etcd.PromotesLearnersLabel: "leader",
			})},
			expected: true,
		},
		{
			name: "older leader labeled with its invocation ID",
			members: []runtime.Object{otherMember, member("etcd-name-differs", map[string]string{
				etcd.ShutdownOnLeaveLabel: "leader",
			})},
			expected: false,
		},
		{
			name:     "older leader without labels",
			members:  []runtime.Object{otherMember, member("controller0", nil)},
			expected: false,
		},
		{
			name: "label from an earlier process",
			members: []runtime.Object{otherMember, member("controller0", map[string]string{
				etcd.ControllerLeaseLabel:  "k0s-ctrl-controller0",
				etcd.PromotesLearnersLabel: "earlier",
			})},
			err: "the leading controller (k0s-ctrl-controller0) hasn't updated its EtcdMember yet",
		},
		{
			name:    "leader has no EtcdMember yet",
			members: []runtime.Object{otherMember},
			err:     "the leading controller (k0s-ctrl-controller0) has no EtcdMember yet",
		},
		{
			name:   "leader has no controller lease",
			leases: []runtime.Object{lease(leaderLeaseName, "leader")},
			err:    "the leading controller leader holds no controller lease",
		},
		{
			name:   "no leader lease",
			leases: []runtime.Object{},
			err:    `failed to get the leader lease: leases.coordination.k8s.io "k0s-endpoint-reconciler" not found`,
		},
		{
			name:   "nobody leads",
			leases: []runtime.Object{lease(leaderLeaseName, "")},
			err:    "no controller is leading",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			if test.leases == nil {
				test.leases = leases
			}
			kubeClient := kubefake.NewClientset(test.leases...)
			k0sClient := k0sfake.NewSimpleClientset(test.members...)

			promotes, err := leaderPromotesLearners(kubeClient, k0sClient.EtcdV1beta1().EtcdMembers())(t.Context())
			if test.err != "" {
				assert.EqualError(t, err, test.err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, test.expected, promotes)
		})
	}
}
