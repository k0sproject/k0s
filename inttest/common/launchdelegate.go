/*
Copyright 2022 k0s authors

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package common

import (
	"context"
	"fmt"
	"runtime"
	"strings"
)

type LaunchMode string

const (
	LaunchModeStandalone LaunchMode = "standalone"
	LaunchModeOpenRC     LaunchMode = "OpenRC"
)

// launchDelegate provides an indirection to the launch operations in
// [FootlooseSuite] so that alternate behavior can be performed.
type launchDelegate interface {
	InitController(ctx context.Context, conn *SSHConnection, k0sArgs ...string) error
	StopController(ctx context.Context, conn *SSHConnection) error
	InitWorker(ctx context.Context, conn *SSHConnection, token string, k0sArgs ...string) error
}

// standaloneLaunchDelegate is a launchDelegate that starts controllers and
// workers in "standalone" mode, i.e. not via some service manager.
type standaloneLaunchDelegate struct {
	k0sFullPath     string
	controllerUmask int
}

var _ launchDelegate = (*standaloneLaunchDelegate)(nil)

// InitController initializes a controller in "standalone" mode, meaning that
// the k0s executable is launched directly (vs. started via a service manager).
func (s *standaloneLaunchDelegate) InitController(ctx context.Context, conn *SSHConnection, k0sArgs ...string) error {
	umaskCmd := ""
	if s.controllerUmask != 0 {
		umaskCmd = fmt.Sprintf("umask %d;", s.controllerUmask)
	}

	// Allow any arch for etcd in smokes
	cmd := fmt.Sprintf("%s ETCD_UNSUPPORTED_ARCH=%s nohup %s controller --debug %s >/tmp/k0s-controller.log 2>&1 &", umaskCmd, runtime.GOARCH, s.k0sFullPath, strings.Join(k0sArgs, " "))

	if _, err := conn.ExecWithOutput(ctx, cmd); err != nil {
		return fmt.Errorf("unable to execute '%s': %w", cmd, err)
	}

	return nil
}

// StopController stops a k0s controller that was started in "standalone" mode.
func (s *standaloneLaunchDelegate) StopController(ctx context.Context, conn *SSHConnection) error {
	stopCommand := fmt.Sprintf("kill $(pidof %s | tr \" \" \"\\n\" | sort -n | head -n1) && while pidof %s; do sleep 0.1s; done", s.k0sFullPath, s.k0sFullPath)
	if _, err := conn.ExecWithOutput(ctx, stopCommand); err != nil {
		return fmt.Errorf("unable to execute '%s': %w", stopCommand, err)
	}

	return nil
}

// InitWorker initializes a worker in "standalone" mode, meaning that the k0s
// executable is launched directly (vs. started via a service manager).
func (s *standaloneLaunchDelegate) InitWorker(ctx context.Context, conn *SSHConnection, token string, k0sArgs ...string) error {
	if token == "" {
		return fmt.Errorf("got empty token for worker join")
	}

	cmd := fmt.Sprintf(`nohup %s worker --debug %s "%s" >/tmp/k0s-worker.log 2>&1 &`, s.k0sFullPath, strings.Join(k0sArgs, " "), token)
	if _, err := conn.ExecWithOutput(ctx, cmd); err != nil {
		return fmt.Errorf("unable to execute '%s': %w", cmd, err)
	}

	return nil
}

// OpenRCLaunchDelegate is a launchDelegate that starts controllers and workers
// via an OpenRC service.
type openRCLaunchDelegate struct {
	k0sFullPath string
}

var _ launchDelegate = (*openRCLaunchDelegate)(nil)

// InitController initializes a controller in "OpenRC" mode, meaning that the
// k0s executable is launched as a service managed by OpenRC.
func (o *openRCLaunchDelegate) InitController(ctx context.Context, conn *SSHConnection, k0sArgs ...string) error {
	if err := o.installK0sService(ctx, conn, "controller"); err != nil {
		return fmt.Errorf("unable to install OpenRC k0s controller: %w", err)
	}

	// Configure k0s as a controller w/args
	controllerArgs := fmt.Sprintf("controller --debug %s", strings.Join(k0sArgs, " "))
	if err := configureK0sServiceArgs(ctx, conn, "controller", controllerArgs); err != nil {
		return fmt.Errorf("failed to configure k0s with '%s'", controllerArgs)
	}

	cmd := "/etc/init.d/k0scontroller start"
	if _, err := conn.ExecWithOutput(ctx, cmd); err != nil {
		return fmt.Errorf("unable to execute '%s': %w", cmd, err)
	}

	return nil
}

// StopController stops a k0s controller that was started using OpenRC.
func (*openRCLaunchDelegate) StopController(ctx context.Context, conn *SSHConnection) error {
	startCmd := "/etc/init.d/k0scontroller stop"
	if _, err := conn.ExecWithOutput(ctx, startCmd); err != nil {
		return fmt.Errorf("unable to execute '%s': %w", startCmd, err)
	}

	return nil
}

// InitWorker initializes a worker in "OpenRC" mode, meaning that the k0s
// executable is launched as a service managed by OpenRC.
func (o *openRCLaunchDelegate) InitWorker(ctx context.Context, conn *SSHConnection, token string, k0sArgs ...string) error {
	if err := o.installK0sService(ctx, conn, "worker"); err != nil {
		return fmt.Errorf("unable to install OpenRC k0s worker: %w", err)
	}

	// Configure k0s as a worker w/args
	workerArgs := fmt.Sprintf("worker --debug %s %s", strings.Join(k0sArgs, " "), token)

	if err := configureK0sServiceArgs(ctx, conn, "worker", workerArgs); err != nil {
		return fmt.Errorf("failed to configure k0s with '%s'", workerArgs)
	}

	cmd := "/etc/init.d/k0sworker start"
	if _, err := conn.ExecWithOutput(ctx, cmd); err != nil {
		return fmt.Errorf("unable to execute '%s': %w", cmd, err)
	}

	return nil
}

// installK0sServiceOpenRC will install an OpenRC k0s-type service (controller/worker)
// if it does not already exist.
func (o *openRCLaunchDelegate) installK0sService(ctx context.Context, conn *SSHConnection, k0sType string) error {
	existsCommand := fmt.Sprintf("/usr/bin/file /etc/init.d/k0s%s", k0sType)
	if _, err := conn.ExecWithOutput(ctx, existsCommand); err != nil {
		cmd := fmt.Sprintf("%s install %s", o.k0sFullPath, k0sType)
		if _, err := conn.ExecWithOutput(ctx, cmd); err != nil {
			return fmt.Errorf("unable to execute '%s': %w", cmd, err)
		}
	}

	return nil
}

// configureK0sServiceArgs performs some reconfiguring of the
// `/etc/init.d/k0s[controller|worker]` startup script to allow for different
// configurations at test time, using the same base image.
func configureK0sServiceArgs(ctx context.Context, conn *SSHConnection, k0sType string, args string) error {
	k0sServiceFile := fmt.Sprintf("/etc/init.d/k0s%s", k0sType)
	cmd := fmt.Sprintf("sed -i 's#^command_args=.*$#command_args=\"%s\"#g' %s", args, k0sServiceFile)

	_, err := conn.ExecWithOutput(ctx, cmd)
	if err != nil {
		return fmt.Errorf("failed to execute '%s' on %s: %w", cmd, conn.Address, err)
	}

	return nil
}
