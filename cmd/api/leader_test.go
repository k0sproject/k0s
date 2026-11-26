// SPDX-FileCopyrightText: 2026 k0s authors
// SPDX-License-Identifier: Apache-2.0

package api

import (
	"errors"
	"testing"

	etcdv1beta1 "github.com/k0sproject/k0s/pkg/apis/etcd/v1beta1"
	k0sfake "github.com/k0sproject/k0s/pkg/client/clientset/fake"
	"github.com/k0sproject/k0s/pkg/etcd"

	coordinationv1 "k8s.io/api/coordination/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	kubefake "k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"
	"k8s.io/utils/ptr"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLeaderPromotesLearners(t *testing.T) {
	leaderLease := func(holder string) runtime.Object {
		return &coordinationv1.Lease{
			ObjectMeta: metav1.ObjectMeta{Name: leaderLeaseName, Namespace: corev1.NamespaceNodeLease},
			Spec:       coordinationv1.LeaseSpec{HolderIdentity: ptr.To(holder)},
		}
	}
	member := func(name string, labels map[string]string) runtime.Object {
		return &etcdv1beta1.EtcdMember{ObjectMeta: metav1.ObjectMeta{Name: name, Labels: labels}}
	}
	// controller1 runs a k0s that promotes learners, under the invocation ID
	// "other". It doesn't lead in any of the cases.
	otherMember := member("controller1", map[string]string{etcd.PromotesLearnersLabel: "other"})

	for _, test := range []struct {
		name     string
		lease    runtime.Object // The leader lease; none if nil.
		leaseErr error          // Returned when getting the leader lease.
		members  []runtime.Object
		expected bool
		err      string
	}{
		{
			name:     "leader promotes learners",
			lease:    leaderLease("leader"),
			members:  []runtime.Object{otherMember, member("controller0", map[string]string{etcd.PromotesLearnersLabel: "leader"})},
			expected: true,
		},
		{
			name:     "leader promotes learners, its EtcdMember is named differently",
			lease:    leaderLease("leader"),
			members:  []runtime.Object{otherMember, member("etcd-name", map[string]string{etcd.PromotesLearnersLabel: "leader"})},
			expected: true,
		},
		{
			name:    "only another controller promotes learners",
			lease:   leaderLease("leader"),
			members: []runtime.Object{otherMember, member("controller0", nil)},
		},
		{
			name:    "leader's label is from an earlier process",
			lease:   leaderLease("leader"),
			members: []runtime.Object{otherMember, member("controller0", map[string]string{etcd.PromotesLearnersLabel: "earlier"})},
		},
		{
			name:    "older leader, its EtcdMember is named differently",
			lease:   leaderLease("leader"),
			members: []runtime.Object{otherMember, member("etcd-name", nil)},
		},
		{
			name:    "leader has no EtcdMember yet",
			lease:   leaderLease("leader"),
			members: []runtime.Object{otherMember},
		},
		{
			name:    "no leader lease, e.g. in single node mode",
			members: []runtime.Object{otherMember},
		},
		{
			name:    "nobody leads",
			lease:   leaderLease(""),
			members: []runtime.Object{otherMember},
		},
		{
			name:     "leader lease can't be read",
			leaseErr: errors.New("injected"),
			err:      "failed to get the leader lease: injected",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			var leases []runtime.Object
			if test.lease != nil {
				leases = append(leases, test.lease)
			}
			kubeClient := kubefake.NewClientset(leases...)
			if test.leaseErr != nil {
				kubeClient.PrependReactor("get", "leases", func(k8stesting.Action) (bool, runtime.Object, error) {
					return true, nil, test.leaseErr
				})
			}
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
