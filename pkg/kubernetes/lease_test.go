// SPDX-FileCopyrightText: 2021 k0s authors
// SPDX-License-Identifier: Apache-2.0

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

	assert.True(t, IsValidLease(lease))
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

	assert.False(t, IsValidLease(lease))
}
