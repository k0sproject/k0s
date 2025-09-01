//go:build linux
// +build linux

// SPDX-FileCopyrightText: 2025 k0s authors
// SPDX-License-Identifier: Apache-2.0

package cgroup

import (
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
	kubeletv1beta1 "k8s.io/kubelet/config/v1beta1"
)

// Ensure implementations satisfy the interface
var _ CgroupManager = (*systemdCgroupManager)(nil)
var _ CgroupManager = (*genericCgroupManager)(nil)

// NewCgroupManager returns a cgroup manager based on detection/upgrade logic.
// If prevConfigPath is non-empty and a previous cgroup driver is found, it will be used for compatibility.
func NewCgroupManager(prevConfigPath string) CgroupManager {
	if prevConfigPath != "" {
		if prev, ok := detectPreviousCgroupDriver(prevConfigPath); ok {
			switch prev {
			case "systemd":
				return &systemdCgroupManager{}
			case "cgroupfs":
				return &genericCgroupManager{}
			}
		}
	}
	if isSystemd() {
		return &systemdCgroupManager{}
	}
	return &genericCgroupManager{}
}

// detectPreviousCgroupDriver tries to read the cgroupDriver from a kubelet config file.
// Returns (driver, true) if config exists, ("", false) if not.
func detectPreviousCgroupDriver(path string) (string, bool) {
	f, err := os.Open(path)
	if err != nil {
		return "", false
	}
	defer f.Close()
	var cfg struct {
		CgroupDriver string `yaml:"cgroupDriver"`
	}
	dec := yaml.NewDecoder(f)
	if err := dec.Decode(&cfg); err != nil {
		// config exists, but couldn't decode, treat as no previous driver
		return "", false
	}
	if cfg.CgroupDriver == "" {
		// config exists, but no previous driver set: kubelet defaults to cgroupfs
		fmt.Printf("Detected previous cgroup driver from %s: (empty, defaulting to cgroupfs)\n", path)
		return "cgroupfs", true
	}
	fmt.Printf("Detected previous cgroup driver from %s: %s\n", path, cfg.CgroupDriver)
	return cfg.CgroupDriver, true
}

// isSystemd detects if systemd is the init system (linux only).
func isSystemd() bool {
	data, err := os.ReadFile("/proc/1/comm")
	if err != nil {
		return false
	}
	return strings.TrimSpace(string(data)) == "systemd"
}

// The implementations follow the previous defaults we've had to ensure compatibility with existing configurations.
// For any real cgroup slice etc changes we need to figure out how to safely migrate existing setups.

// Systemd implementation
type systemdCgroupManager struct{}

// Driver implements CgroupManager.
func (s *systemdCgroupManager) Driver() string {
	return "systemd"
}

func (s *systemdCgroupManager) ApplyToConfig(cfg *kubeletv1beta1.KubeletConfiguration) {
	if cfg == nil {
		return
	}
	if cfg.CgroupDriver == "" {
		cfg.CgroupDriver = "systemd"
	}
	if cfg.KubeletCgroups == "" {
		cfg.KubeletCgroups = "/system.slice/containerd.service"
	}
	if cfg.KubeReservedCgroup == "" {
		cfg.KubeReservedCgroup = "system.slice"
	}
}

// Generic implementation for non-systemd setups
type genericCgroupManager struct{}

// Driver implements CgroupManager.
func (g *genericCgroupManager) Driver() string {
	return "cgroupfs"
}

func (g *genericCgroupManager) ApplyToConfig(cfg *kubeletv1beta1.KubeletConfiguration) {
	if cfg == nil {
		return
	}
	if cfg.CgroupDriver == "" {
		cfg.CgroupDriver = "cgroupfs"
	}
	if cfg.KubeletCgroups == "" {
		cfg.KubeletCgroups = "/system.slice/containerd.service"
	}
	if cfg.KubeReservedCgroup == "" {
		cfg.KubeReservedCgroup = "system.slice"
	}
}
