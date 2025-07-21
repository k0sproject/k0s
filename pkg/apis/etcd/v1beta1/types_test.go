// SPDX-FileCopyrightText: 2024 k0s authors
// SPDX-License-Identifier: Apache-2.0

package v1beta1

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestStatus_SetConditionWhenEmpty(t *testing.T) {
	em := &EtcdMember{}

	em.Status.SetCondition(ConditionTypeJoined, ConditionTrue, "foobar", time.Now())

	r := require.New(t)
	r.Len(em.Status.Conditions, 1)
	r.NotEmpty(em.Status.Conditions[0].LastTransitionTime)
}

func TestStatus_SetConditionChange(t *testing.T) {
	start := time.Now()
	em := &EtcdMember{
		Status: Status{
			Conditions: []JoinCondition{
				{
					Type:               ConditionTypeJoined,
					Status:             ConditionTrue,
					LastTransitionTime: metav1.NewTime(start),
				},
			},
		},
	}

	em.Status.SetCondition(ConditionTypeJoined, ConditionFalse, "foobar", start.Add(12*time.Second))

	r := require.New(t)
	c := em.Status.Conditions[0]
	r.Len(em.Status.Conditions, 1)
	t.Logf("original time: %s", metav1.NewTime(start))
	t.Logf("latest time: %s", c.LastTransitionTime.Time)
	r.True(start.Before(c.LastTransitionTime.Time))
	r.Equal(ConditionFalse, c.Status)
}
