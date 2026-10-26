// SPDX-FileCopyrightText: 2026 k0s authors
// SPDX-License-Identifier: Apache-2.0

package sysservice

import (
	"context"
	"errors"
	"fmt"
	"os"
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

func newWindows(name string) *windowsService {
	displayName := name
	if suffix := strings.TrimPrefix(name, "k0s"); suffix != "" {
		displayName = "k0s " + suffix
	}
	return &windowsService{name: name, displayName: displayName}
}

func (s *windowsService) Install(ctx context.Context, opts InstallOpts) (retErr error) {
	m, err := mgr.Connect()
	if err != nil {
		return fmt.Errorf("connect to SCM: %w", err)
	}
	defer func() { _ = m.Disconnect() }()

	exec, err := os.Executable()
	if err != nil {
		return fmt.Errorf("get executable: %w", err)
	}

	// Prepend "service=<name>" so k0s knows it's running as a Windows service.
	args := append([]string{"service=" + s.name}, opts.Args...)

	sv, err := m.CreateService(s.name, exec, mgr.Config{
		DisplayName: s.displayName,
		Description: "k0s - Zero Friction Kubernetes",
		StartType:   mgr.StartManual,
	}, args...)
	switch {
	case err == nil:
		// The service is ours, so remove it again if anything below fails.
		defer func() {
			if retErr != nil {
				_ = sv.Delete()
			}
			sv.Close()
		}()

	case !errors.Is(err, windows.ERROR_SERVICE_EXISTS):
		return fmt.Errorf("create service: %w", err)

	case !opts.Force:
		return fmt.Errorf("%w: %s", ErrAlreadyInstalled, s.name)

	default:
		// Reconfigure the existing service rather than deleting and recreating
		// it. Deletion is asynchronous for as long as anyone holds a handle to
		// the service, which would make the subsequent create fail.
		if sv, err = m.OpenService(s.name); err != nil {
			return fmt.Errorf("open service: %w", err)
		}
		defer sv.Close()

		config, err := sv.Config()
		if err != nil {
			return fmt.Errorf("get service config: %w", err)
		}
		config.BinaryPathName = windows.ComposeCommandLine(append([]string{exec}, args...))
		config.DisplayName = s.displayName
		if err := sv.UpdateConfig(config); err != nil {
			return fmt.Errorf("update service config: %w", err)
		}
	}

	// Set the environment via the service's registry key. The SCM has no API
	// for this. The key is removed along with the service.
	key, err := registry.OpenKey(registry.LOCAL_MACHINE,
		`SYSTEM\CurrentControlSet\Services\`+s.name,
		registry.SET_VALUE)
	if err != nil {
		return fmt.Errorf("open service registry key: %w", err)
	}
	defer key.Close()
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

func (s *windowsService) Uninstall(ctx context.Context) error {
	m, err := mgr.Connect()
	if err != nil {
		return fmt.Errorf("connect to SCM: %w", err)
	}
	defer func() { _ = m.Disconnect() }()

	sv, err := m.OpenService(s.name)
	if err != nil {
		if errors.Is(err, windows.ERROR_SERVICE_DOES_NOT_EXIST) {
			return fmt.Errorf("%w: %s", ErrNotInstalled, s.name)
		}
		return fmt.Errorf("open service: %w", err)
	}
	defer sv.Close()

	if err := stopAndWait(ctx, sv); err != nil {
		return err
	}

	return sv.Delete()
}

func (s *windowsService) Enable(ctx context.Context) error {
	return s.withService(ctx, func(sv *mgr.Service) error {
		return setStartType(sv, mgr.StartAutomatic)
	})
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

func (s *windowsService) Status(ctx context.Context) (Status, error) {
	m, err := mgr.Connect()
	if err != nil {
		return StatusUnknown, fmt.Errorf("connect to SCM: %w", err)
	}
	defer func() { _ = m.Disconnect() }()

	sv, err := m.OpenService(s.name)
	if err != nil {
		if errors.Is(err, windows.ERROR_SERVICE_DOES_NOT_EXIST) {
			return StatusNotInstalled, nil
		}
		return StatusUnknown, fmt.Errorf("open service: %w", err)
	}
	defer sv.Close()

	status, err := sv.Query()
	if err != nil {
		return StatusUnknown, fmt.Errorf("query service: %w", err)
	}

	switch status.State {
	case svc.Running, svc.StartPending:
		return StatusRunning, nil
	case svc.Stopped:
		return StatusStopped, nil
	default:
		return StatusUnknown, nil
	}
}

// withService connects to the SCM, opens the named service, calls fn, then
// closes both handles.
func (s *windowsService) withService(ctx context.Context, fn func(*mgr.Service) error) error {
	m, err := mgr.Connect()
	if err != nil {
		return fmt.Errorf("connect to SCM: %w", err)
	}
	defer func() { _ = m.Disconnect() }()

	if err := ctx.Err(); err != nil {
		return err
	}

	sv, err := m.OpenService(s.name)
	if err != nil {
		return fmt.Errorf("open service: %w", err)
	}
	defer sv.Close()

	return fn(sv)
}

func setStartType(sv *mgr.Service, startType uint32) error {
	config, err := sv.Config()
	if err != nil {
		return fmt.Errorf("get service config: %w", err)
	}
	config.StartType = startType
	return sv.UpdateConfig(config)
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
			return ctx.Err()
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
