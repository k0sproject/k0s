// SPDX-FileCopyrightText: 2021 k0s authors
// SPDX-License-Identifier: Apache-2.0

package kubernetes

import (
	"context"
	"strings"
	"time"

	coordinationv1 "k8s.io/api/coordination/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// IsValidLease check whether or not the lease is expired
func IsValidLease(lease coordinationv1.Lease) bool {
	leaseDur := time.Duration(*lease.Spec.LeaseDurationSeconds)

	leaseExpiry := lease.Spec.RenewTime.Add(leaseDur * time.Second)

	return leaseExpiry.After(time.Now())
}

func CountActiveControllerLeases(ctx context.Context, kubeClient kubernetes.Interface) (count uint, _ error) {
	leases, err := kubeClient.CoordinationV1().Leases(corev1.NamespaceNodeLease).List(ctx, metav1.ListOptions{})
	if err != nil {
		return 0, err
	}
	for _, l := range leases.Items {
		switch {
		case !strings.HasPrefix(l.Name, "k0s-ctrl-"):
		case l.Spec.HolderIdentity == nil || *l.Spec.HolderIdentity == "":
		case IsValidLease(l):
			count++
		}
	}

	return count, nil
}
