// SPDX-FileCopyrightText: 2021 k0s authors
// SPDX-License-Identifier: Apache-2.0

package kubernetes

import (
	"time"

	coordinationv1 "k8s.io/api/coordination/v1"
)

// IsValidLease check whether or not the lease is expired
func IsValidLease(lease coordinationv1.Lease) bool {
	leaseDur := time.Duration(*lease.Spec.LeaseDurationSeconds)

	leaseExpiry := lease.Spec.RenewTime.Add(leaseDur * time.Second)

	return leaseExpiry.After(time.Now())
}
