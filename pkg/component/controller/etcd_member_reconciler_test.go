// SPDX-FileCopyrightText: 2026 k0s authors
// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"maps"
	"testing"

	etcdv1beta1 "github.com/k0sproject/k0s/pkg/apis/etcd/v1beta1"
	k0sv1beta1 "github.com/k0sproject/k0s/pkg/apis/k0s/v1beta1"
	"github.com/k0sproject/k0s/pkg/etcd"

	coordinationv1 "k8s.io/api/coordination/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/k0sproject/k0s/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEtcdMemberReconciler_ControllerLeaseLookup(t *testing.T) {
	const invocationID = "0123456789abcdef0123456789abcdef"

	for _, tt := range []struct {
		name        string
		annotations map[string]string
		labels      map[string]string
		leaseName   string
	}{
		{
			"uses the annotation, even if there's a label",
			map[string]string{"k0s.k0sproject.io/controller-lease": "k0s-ctrl-annotated"},
			map[string]string{"k0s.k0sproject.io/controller-lease": "k0s-ctrl-labeled"},
			"k0s-ctrl-annotated",
		},
		{
			"uses the label if there's no annotation",
			nil,
			map[string]string{"k0s.k0sproject.io/controller-lease": "k0s-ctrl-labeled"},
			"k0s-ctrl-labeled",
		},
		{
			"derives the name from the member name if there's neither",
			nil,
			nil,
			"k0s-ctrl-foo",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			labels := map[string]string{"k0s.k0sproject.io/shutdown-on-leave": invocationID}
			maps.Copy(labels, tt.labels)
			member := &etcdv1beta1.EtcdMember{
				Name:        "foo",
				Labels:      labels,
				Annotations: tt.annotations,
				Spec:        etcdv1beta1.EtcdMemberSpec{Leave: true},
				Status:      etcdv1beta1.Status{MemberID: "1", PeerAddress: "10.0.0.1"},
			}
			now := metav1.NowMicro()
			lease := &coordinationv1.Lease{
				Namespace: corev1.NamespaceNodeLease,
				Name:      tt.leaseName,
				Spec: coordinationv1.LeaseSpec{
					HolderIdentity:       new(invocationID),
					LeaseDurationSeconds: new(int32(60)),
					AcquireTime:          &now,
					RenewTime:            &now,
				},
			}

			factory := testutil.NewFakeClientFactory(member, lease)
			clients, err := factory.GetK0sClient()
			require.NoError(t, err)
			client := clients.EtcdV1beta1().EtcdMembers()
			underTest := &EtcdMemberReconciler{
				clientFactory: factory,
				etcdConfig:    &k0sv1beta1.EtcdConfig{PeerAddress: "10.0.0.2"},
			}

			done := underTest.reconcileMember(t.Context(), nil, client, []etcd.Member{{ID: 1}}, member)

			assert.True(t, done, "Reconciliation didn't succeed")
			member, err = client.Get(t.Context(), member.Name, metav1.GetOptions{})
			require.NoError(t, err)
			assert.Equal(t, invocationID, member.Labels["k0s.k0sproject.io/shutdown"], "Member shutdown not requested")
		})
	}
}
