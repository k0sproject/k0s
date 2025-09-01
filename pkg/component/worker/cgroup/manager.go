// SPDX-FileCopyrightText: 2025 k0s authors
// SPDX-License-Identifier: Apache-2.0

package cgroup

import (
	kubeletv1beta1 "k8s.io/kubelet/config/v1beta1"
)

// CgroupManager abstracts cgroup driver logic for different init systems.
type CgroupManager interface {
	// ApplyToConfig applies cgroup driver/slice defaults to the config if not set
	ApplyToConfig(cfg *kubeletv1beta1.KubeletConfiguration)
	Driver() string
}
