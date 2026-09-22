// SPDX-FileCopyrightText: 2026 k0s authors
// SPDX-License-Identifier: Apache-2.0

package leasecounter

import (
	"context"
	"strings"

	kubeutil "github.com/k0sproject/k0s/pkg/kubernetes"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// Counts the active controller leases.
func ActiveLeases(ctx context.Context, kubeClient kubernetes.Interface) (count uint, _ error) {
	leases, err := kubeClient.CoordinationV1().Leases(corev1.NamespaceNodeLease).List(ctx, metav1.ListOptions{})
	if err != nil {
		return 0, err
	}
	for _, l := range leases.Items {
		switch {
		// FIXME: Remove the name prefix check in k0s 1.38+.
		case l.Labels[leaseTypeLabel] != leaseTypeLabelValueController && !strings.HasPrefix(l.Name, "k0s-ctrl-"):
		case l.Spec.HolderIdentity == nil || *l.Spec.HolderIdentity == "":
		case kubeutil.IsValidLease(l):
			count++
		}
	}

	return count, nil
}
