//go:build windows

// SPDX-FileCopyrightText: 2025 k0s authors
// SPDX-License-Identifier: Apache-2.0

package cgroup

import kubeletv1beta1 "k8s.io/kubelet/config/v1beta1"

// Ensure implementation satisfies the interface
var _ CgroupManager = (*windowsCgroupManager)(nil)

// CgroupManager no-op implementation for Windows.
type windowsCgroupManager struct{}

// Driver implements CgroupManager.
func (w *windowsCgroupManager) Driver() string {
	panic("unimplemented")
}

func NewCgroupManager(_ string) CgroupManager {
	return &windowsCgroupManager{}
}

func (w *windowsCgroupManager) ApplyToConfig(cfg *kubeletv1beta1.KubeletConfiguration) {
	// no-op for windows
}
