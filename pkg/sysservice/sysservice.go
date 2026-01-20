// SPDX-FileCopyrightText: 2026 k0s authors
// SPDX-License-Identifier: Apache-2.0

package sysservice

import (
	"context"
	"errors"
	"fmt"
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

var (
	// ErrNotInstalled is returned when the service does not exist.
	ErrNotInstalled = errors.New("service is not installed")

	// ErrAlreadyInstalled is returned when the service already exists.
	ErrAlreadyInstalled = errors.New("service is already installed")

	// ErrUnsupportedInitSystem is returned when k0s has no service backend for
	// the host's init system.
	ErrUnsupportedInitSystem = errors.New("no supported init system detected")
)

type Service interface {
	// Install registers the service with the service manager. It fails with an
	// error wrapping [ErrAlreadyInstalled] if the service already exists.
	Install(ctx context.Context, args []string, env []string) error

	// Uninstall removes the service from the service manager. It fails with an
	// error wrapping [ErrNotInstalled] if the service does not exist.
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

	return nil, fmt.Errorf("%w; run k0s from a service definition of your own", ErrUnsupportedInitSystem)
}
