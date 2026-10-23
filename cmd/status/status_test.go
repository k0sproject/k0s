// SPDX-FileCopyrightText: 2026 k0s authors
// SPDX-License-Identifier: Apache-2.0

package status

import (
	"bytes"
	"testing"

	"github.com/k0sproject/k0s/pkg/component/status"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
)

func TestPrintStatus_Conditions(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	printStatus(&buf, &status.K0sStatus{
		Version:                     "v1.34.0+k0s.0",
		Pid:                         42,
		Role:                        "controller",
		Workloads:                   true,
		SingleNode:                  true,
		WorkerToAPIConnectionStatus: status.ProbeStatus{Success: true},
		Conditions: []status.Condition{{
			Type:    "CNIReady",
			Status:  corev1.ConditionTrue,
			Reason:  "AllReady",
			Message: "kube-router DaemonSet reports 1/1 available, 1 updated",
		}},
	}, "")

	assert.Equal(t, `Version: v1.34.0+k0s.0
Process ID: 42
Role: controller
Workloads: true
SingleNode: true
Kube-api probing successful: true
Kube-api probing last error:  
CNIReady: True
Reason: AllReady
Message: kube-router DaemonSet reports 1/1 available, 1 updated
`, buf.String())
}
