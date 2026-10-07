// SPDX-FileCopyrightText: 2026 k0s authors
// SPDX-License-Identifier: Apache-2.0

package utils

import (
	"testing"

	apv1beta2 "github.com/k0sproject/k0s/pkg/apis/autopilot/v1beta2"
	"github.com/k0sproject/k0s/pkg/constant"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	crcli "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/stretchr/testify/assert"
)

func TestIsNodeWithoutK0s(t *testing.T) {
	node := func(version string, labels map[string]string) *corev1.Node {
		return &corev1.Node{
			ObjectMeta: metav1.ObjectMeta{Labels: labels},
			Status:     corev1.NodeStatus{NodeInfo: corev1.NodeSystemInfo{KubeletVersion: version}},
		}
	}
	for _, test := range []struct {
		obj  crcli.Object
		name string
		want bool
	}{
		{node("v1.36.4", map[string]string{constant.K0sNodeLabel: "false"}), "node without k0s", true},
		{node("v1.36.4+k0s", map[string]string{constant.K0sNodeLabel: "true"}), "k0s worker", false},
		// k0s may run upstream kubelets, which report no k0s version suffix.
		{node("v1.36.4", nil), "k0s worker with an upstream kubelet", false},
		{node("", nil), "unreported version", false},
		{&apv1beta2.ControlNode{}, "controller", false},
	} {
		t.Run(test.name, func(t *testing.T) {
			assert.Equal(t, test.want, IsNodeWithoutK0s(test.obj))
		})
	}
}
