// SPDX-FileCopyrightText: 2026 k0s authors
// SPDX-License-Identifier: Apache-2.0

package utils

import (
	"github.com/k0sproject/k0s/pkg/constant"

	crcli "sigs.k8s.io/controller-runtime/pkg/client"
)

// IsNodeWithoutK0s reports whether the object is a node that k0s labeled as not running k0s.
// Autopilot can't update nodes without k0s.
func IsNodeWithoutK0s(obj crcli.Object) bool {
	return obj.GetLabels()[constant.K0sNodeLabel] == "false"
}
