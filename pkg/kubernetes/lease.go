/*
Copyright 2021 k0s authors

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package kubernetes

import (
	"context"
	"strings"
	"time"

	coordinationv1 "k8s.io/api/coordination/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// IsValidLease check whether or not the lease is expired
func IsValidLease(lease coordinationv1.Lease) bool {
	leaseDur := time.Duration(*lease.Spec.LeaseDurationSeconds)

	leaseExpiry := lease.Spec.RenewTime.Add(leaseDur * time.Second)

	return leaseExpiry.After(time.Now())
}

func CountActiveControllerLeases(ctx context.Context, kubeClient kubernetes.Interface) (count uint, _ error) {
	leases, err := kubeClient.CoordinationV1().Leases("kube-node-lease").List(ctx, v1.ListOptions{})
	if err != nil {
		return 0, err
	}
	for _, l := range leases.Items {
		switch {
		case !strings.HasPrefix(l.ObjectMeta.Name, "k0s-ctrl-"):
		case l.Spec.HolderIdentity == nil || *l.Spec.HolderIdentity == "":
		case IsValidLease(l):
			count++
		}
	}

	return count, nil
}
