//go:build !windows

// SPDX-FileCopyrightText: 2026 k0s authors
// SPDX-License-Identifier: Apache-2.0

package sysservice

import (
	"errors"
	"os"
	"os/exec"
)

// New returns the service with the given name, managed by the host's init
// system. Fails with [ErrUnsupportedInitSystem] if the init system is neither
// systemd nor OpenRC.
func New(name string) (Service, error) {
	if name == "" {
		return nil, errors.New("service name must not be empty")
	}

	// Prefer filesystem markers (most reliable), then fall back to PATH checks.
	if _, err := os.Stat("/run/systemd/system"); err == nil {
		return newSystemd(name), nil
	}
	if _, err := os.Stat("/run/openrc"); err == nil {
		return newOpenRC(name), nil
	}
	if _, err := exec.LookPath("systemctl"); err == nil {
		return newSystemd(name), nil
	}
	if _, err := exec.LookPath("openrc-init"); err == nil {
		return newOpenRC(name), nil
	}
	if _, err := exec.LookPath("rc-service"); err == nil {
		return newOpenRC(name), nil
	}

	return nil, ErrUnsupportedInitSystem
}
