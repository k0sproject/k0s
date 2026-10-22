// SPDX-FileCopyrightText: 2026 k0s authors
// SPDX-License-Identifier: Apache-2.0

package leasecounter

import (
	"context"
	"strings"

	kubeutil "github.com/k0sproject/k0s/pkg/kubernetes"

	coordinationv1 "k8s.io/api/coordination/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// Returns the active controller leases.
func ActiveLeases(ctx context.Context, kubeClient kubernetes.Interface) (activeLeases []coordinationv1.Lease, _ error) {
	allLeases, err := controllerLeases(ctx, kubeClient)
	if err != nil {
		return nil, err
	}

	for _, l := range allLeases {
		if isActive(&l) {
			activeLeases = append(activeLeases, l)
		}
	}

	return
}

// Returns all controller leases, active or not.
func controllerLeases(ctx context.Context, kubeClient kubernetes.Interface) (leases []coordinationv1.Lease, _ error) {
	allLeases, err := kubeClient.CoordinationV1().Leases(corev1.NamespaceNodeLease).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	for _, l := range allLeases.Items {
		// FIXME: Remove the name prefix check in k0s 1.38+.
		if l.Labels[leaseTypeLabel] == leaseTypeLabelValueController || strings.HasPrefix(l.Name, "k0s-ctrl-") {
			leases = append(leases, l)
		}
	}

	return
}

// Indicates if the lease is held and hasn't expired.
func isActive(l *coordinationv1.Lease) bool {
	return l.Spec.HolderIdentity != nil && *l.Spec.HolderIdentity != "" && kubeutil.IsValidLease(*l)
}
