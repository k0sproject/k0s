//go:build !linux && !windows
// +build !linux,!windows

// SPDX-FileCopyrightText: 2025 k0s authors
// SPDX-License-Identifier: Apache-2.0

package cgroup

import kubeletv1beta1 "k8s.io/kubelet/config/v1beta1"

// Ensure implementation satisfies the interface
var _ CgroupManager = (*stubCgroupManager)(nil)

// CgroupManager stub for unsupported platforms (so the package always builds)
// CgroupManager stub for unsupported platforms (so the package always builds)
type stubCgroupManager struct{}

// Driver implements CgroupManager.
func (s *stubCgroupManager) Driver() string {
	panic("unimplemented")
}

// NewCgroupManager returns a no-op manager for unsupported platforms.
func NewCgroupManager(_ string) CgroupManager {
	return &stubCgroupManager{}
}

func (s *stubCgroupManager) ApplyToConfig(cfg *kubeletv1beta1.KubeletConfiguration) {
	// no-op for stub
}
