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
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	coordination "k8s.io/api/coordination/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestValidLease(t *testing.T) {
	leaseDuration := int32(60)
	microNow := metav1.NowMicro()

	lease := coordination.Lease{
		Spec: coordination.LeaseSpec{
			LeaseDurationSeconds: &leaseDuration,
			RenewTime:            &microNow,
		},
	}

	assert.Equal(t, true, IsValidLease(lease))
}

func TestExpiredLease(t *testing.T) {
	leaseDuration := int32(60)
	renew := metav1.NewMicroTime(time.Now().Add(-62 * time.Second))

	lease := coordination.Lease{
		Spec: coordination.LeaseSpec{
			LeaseDurationSeconds: &leaseDuration,
			RenewTime:            &renew,
		},
	}

	assert.Equal(t, false, IsValidLease(lease))
}
