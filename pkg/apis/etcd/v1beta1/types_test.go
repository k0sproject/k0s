/*
Copyright 2024 k0s authors

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
