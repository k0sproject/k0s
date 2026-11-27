// SPDX-FileCopyrightText: 2026 k0s authors
// SPDX-License-Identifier: Apache-2.0

package sysservice

import (
	"context"
	"errors"
	"os"

	"github.com/k0sproject/k0s/internal/pkg/file"
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

// InstallOpts describes the service to be installed.
type InstallOpts struct {
	// Executable is the absolute path of the k0s executable.
	Executable string

	// Args are the arguments to the k0s executable.
	Args []string

	// Env are the environment variables for the service, as KEY=value.
	Env []string

	// Force replaces the definition of an already installed service instead of
	// failing.
	Force bool
}

type Service interface {
	// Install registers the service with the service manager, so that it starts
	// automatically on boot. It fails with an error wrapping
	// [ErrAlreadyInstalled] if the service already exists and
	// [InstallOpts.Force] isn't set. It never starts or stops the service, so a
	// replaced definition only takes effect once the service is restarted.
	Install(ctx context.Context, opts InstallOpts) error

	// Uninstall removes the service from the service manager, including from
	// the services started on boot. It fails with an error wrapping
	// [ErrNotInstalled] if the service does not exist.
	Uninstall(ctx context.Context) error

	// Start starts the service. It fails if the service is not installed.
	Start(ctx context.Context) error

	// Stop stops the service. It succeeds if the service is already stopped.
	Stop(ctx context.Context) error

	// Status returns the current status of the service.
	Status(ctx context.Context) (Status, error)
}

// writeDefinition atomically writes a service definition file. Fails with an
// error wrapping [fs.ErrExist] if the file exists already, unless replace is
// set.
func writeDefinition(path string, content []byte, perm os.FileMode, replace bool) error {
	f := file.AtomicWithTarget(path).WithPermissions(perm)
	if !replace {
		f = f.WithNoReplace()
	}
	return f.Write(content)
}
