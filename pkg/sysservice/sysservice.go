// SPDX-FileCopyrightText: 2026 k0s authors
// SPDX-License-Identifier: Apache-2.0

package sysservice

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"runtime"
)

type Status int

const (
	StatusUnknown Status = iota
	StatusNotInstalled
	StatusStopped
	StatusRunning
)

type Service interface {
	// Install registers the service with the service manager. It fails with an
	// error wrapping [fs.ErrExist] if the service is already installed.
	Install(ctx context.Context, args []string, env []string) error

	// Uninstall removes the service from the service manager. It fails with an
	// error wrapping [os.ErrNotExist] if the service is not installed.
	Uninstall(ctx context.Context) error

	// Enable configures the service to start automatically on boot.
	Enable(ctx context.Context) error

	// Start starts the service. It fails if the service is not installed.
	Start(ctx context.Context) error

	// Stop stops the service. It succeeds if the service is already stopped.
	Stop(ctx context.Context) error

	// Status returns the current status of the service.
	Status(ctx context.Context) (Status, error)
}

func New(name string) (Service, error) {
	if name == "" {
		return nil, errors.New("service name must not be empty")
	}

	if runtime.GOOS == "windows" {
		return newWindows(name), nil
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

	return nil, errors.New("unable to detect init/service manager (systemd/openrc)")
}
