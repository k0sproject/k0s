/*
Copyright 2020 Mirantis, Inc.

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
	"time"

	coordinationv1 "k8s.io/api/coordination/v1"
)

// IsValidLease check whether or not the lease is expired
func IsValidLease(lease coordinationv1.Lease) bool {
	leaseDur := time.Duration(*lease.Spec.LeaseDurationSeconds)

	leaseExpiry := lease.Spec.RenewTime.Add(leaseDur * time.Second)

	return leaseExpiry.After(time.Now())
}
