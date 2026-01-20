// SPDX-FileCopyrightText: 2026 k0s authors
// SPDX-License-Identifier: Apache-2.0

package sysservice

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
	"golang.org/x/sys/windows/svc"
	"golang.org/x/sys/windows/svc/mgr"
)

type windowsService struct {
	name        string
	displayName string
}

// New returns the service with the given name, managed by the Windows Service
// Control Manager.
func New(name string) (Service, error) {
	if name == "" {
		return nil, errors.New("service name must not be empty")
	}

	displayName := name
	if suffix := strings.TrimPrefix(name, "k0s"); suffix != "" {
		displayName = "k0s " + suffix
	}
	return &windowsService{name: name, displayName: displayName}, nil
}

func (s *windowsService) Install(ctx context.Context, opts InstallOpts) (err error) {
	m, err := mgr.Connect()
	if err != nil {
		return fmt.Errorf("connect to SCM: %w", err)
	}
	defer func() { err = errors.Join(err, m.Disconnect()) }()

	// Prepend "service=<name>" so k0s knows it's running as a Windows service.
	args := append([]string{"service=" + s.name}, opts.Args...)

	sv, created, err := createOrUpdateService(m, s.name, opts.Executable, mgr.Config{
		DisplayName: s.displayName,
		Description: "k0s - Zero Friction Kubernetes",
		StartType:   mgr.StartAutomatic,
	}, opts.Force, args...)
	if err != nil {
		return err
	}
	defer func() {
		// A newly created service is ours, so remove it again if anything
		// below fails.
		if err != nil && created {
			if delErr := sv.Delete(); !errors.Is(delErr, windows.ERROR_SERVICE_MARKED_FOR_DELETE) {
				err = errors.Join(err, delErr)
			}
		}
		err = errors.Join(err, sv.Close())
	}()

	// Set the environment via the service's registry key. The SCM has no API
	// for this. The key is removed along with the service.
	key, err := registry.OpenKey(registry.LOCAL_MACHINE,
		`SYSTEM\CurrentControlSet\Services\`+s.name,
		registry.SET_VALUE)
	if err != nil {
		return fmt.Errorf("open service registry key: %w", err)
	}
	defer func() { err = errors.Join(err, key.Close()) }()
	if len(opts.Env) > 0 {
		if err := key.SetStringsValue("Environment", opts.Env); err != nil {
			return fmt.Errorf("set service environment: %w", err)
		}
	} else if err := key.DeleteValue("Environment"); err != nil && !errors.Is(err, registry.ErrNotExist) {
		// Don't leak the environment of a previous installation.
		return fmt.Errorf("delete service environment: %w", err)
	}

	return nil
}

// createOrUpdateService creates the named service. If it exists already and
// force is set, it updates the existing service's command line, display name
// and start type instead. Returns whether the service has been newly created.
func createOrUpdateService(m *mgr.Mgr, name, exepath string, c mgr.Config, force bool, args ...string) (_ *mgr.Service, created bool, _ error) {
	sv, err := m.CreateService(name, exepath, c, args...)
	if err == nil {
		return sv, true, nil
	}
	if !errors.Is(err, windows.ERROR_SERVICE_EXISTS) {
		return nil, false, fmt.Errorf("create service: %w", err)
	}
	if !force {
		return nil, false, fmt.Errorf("%w: %s", ErrAlreadyInstalled, name)
	}

	// Reconfigure the existing service rather than deleting and recreating
	// it. Deletion is asynchronous for as long as anyone holds a handle to
	// the service, which would make the subsequent create fail.
	if sv, err = m.OpenService(name); err != nil {
		return nil, false, fmt.Errorf("open service: %w", err)
	}
	config, err := sv.Config()
	if err != nil {
		return nil, false, errors.Join(fmt.Errorf("get service config: %w", err), sv.Close())
	}
	config.BinaryPathName = windows.ComposeCommandLine(append([]string{exepath}, args...))
	config.DisplayName = c.DisplayName
	config.StartType = c.StartType
	if err := sv.UpdateConfig(config); err != nil {
		return nil, false, errors.Join(fmt.Errorf("update service config: %w", err), sv.Close())
	}
	return sv, false, nil
}

func (s *windowsService) Uninstall(ctx context.Context) (err error) {
	m, err := mgr.Connect()
	if err != nil {
		return fmt.Errorf("connect to SCM: %w", err)
	}
	defer func() { err = errors.Join(err, m.Disconnect()) }()

	sv, err := m.OpenService(s.name)
	if err != nil {
		if errors.Is(err, windows.ERROR_SERVICE_DOES_NOT_EXIST) {
			return fmt.Errorf("%w: %s", ErrNotInstalled, s.name)
		}
		return fmt.Errorf("open service: %w", err)
	}
	defer func() { err = errors.Join(err, sv.Close()) }()

	if err := stopAndWait(ctx, sv); err != nil {
		return err
	}

	return sv.Delete()
}

func (s *windowsService) Start(ctx context.Context) error {
	return s.withService(ctx, func(sv *mgr.Service) error {
		return sv.Start()
	})
}

func (s *windowsService) Stop(ctx context.Context) error {
	return s.withService(ctx, func(sv *mgr.Service) error {
		return stopAndWait(ctx, sv)
	})
}

func (s *windowsService) Status(ctx context.Context) (_ Status, err error) {
	m, err := mgr.Connect()
	if err != nil {
		return StatusUnknown, fmt.Errorf("connect to SCM: %w", err)
	}
	defer func() { err = errors.Join(err, m.Disconnect()) }()

	sv, err := m.OpenService(s.name)
	if err != nil {
		if errors.Is(err, windows.ERROR_SERVICE_DOES_NOT_EXIST) {
			return StatusNotInstalled, nil
		}
		return StatusUnknown, fmt.Errorf("open service: %w", err)
	}
	defer func() { err = errors.Join(err, sv.Close()) }()

	status, err := sv.Query()
	if err != nil {
		return StatusUnknown, fmt.Errorf("query service: %w", err)
	}

	switch status.State {
	case svc.Running, svc.StartPending:
		return StatusRunning, nil
	case svc.StopPending:
		// The process is still running but shutting down. Treat as running so
		// that a repeated stop waits for the shutdown to complete, the same as
		// for OpenRC.
		return StatusRunning, nil
	case svc.Stopped:
		return StatusStopped, nil
	default:
		return StatusUnknown, nil
	}
}

// withService connects to the SCM, opens the named service, calls fn, then
// closes both handles.
func (s *windowsService) withService(ctx context.Context, fn func(*mgr.Service) error) (err error) {
	m, err := mgr.Connect()
	if err != nil {
		return fmt.Errorf("connect to SCM: %w", err)
	}
	defer func() { err = errors.Join(err, m.Disconnect()) }()

	if err := context.Cause(ctx); err != nil {
		return err
	}

	sv, err := m.OpenService(s.name)
	if err != nil {
		return fmt.Errorf("open service: %w", err)
	}
	defer func() { err = errors.Join(err, sv.Close()) }()

	return fn(sv)
}

func stopAndWait(ctx context.Context, sv *mgr.Service) error {
	if _, err := sv.Control(svc.Stop); err != nil {
		if errors.Is(err, windows.ERROR_SERVICE_NOT_ACTIVE) {
			return nil
		}
		return err
	}
	for {
		select {
		case <-ctx.Done():
			return context.Cause(ctx)
		default:
		}
		status, err := sv.Query()
		if err != nil {
			return err
		}
		if status.State == svc.Stopped {
			return nil
		}
		time.Sleep(200 * time.Millisecond)
	}
}
