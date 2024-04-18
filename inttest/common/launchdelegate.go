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
	"errors"
	"fmt"
	"io"
	"runtime"
	"strings"
	"sync"
)

type LaunchMode string

const (
	LaunchModeStandalone LaunchMode = "standalone"
	LaunchModeOpenRC     LaunchMode = "OpenRC"
)

// launchDelegate provides an indirection to the launch operations in
// [BootlooseSuite] so that alternate behavior can be performed.
type launchDelegate interface {
	InitController(ctx context.Context, conn *SSHConnection, k0sArgs ...string) error
	StartController(ctx context.Context, conn *SSHConnection) error
	StopController(ctx context.Context, conn *SSHConnection) error
	InitWorker(ctx context.Context, conn *SSHConnection, token string, k0sArgs ...string) error
	StartWorker(ctx context.Context, conn *SSHConnection) error
	StopWorker(ctx context.Context, conn *SSHConnection) error
	ReadK0sLogs(ctx context.Context, conn *SSHConnection, out, err io.Writer) error
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
	var script strings.Builder
	fmt.Fprintln(&script, "#!/usr/bin/env bash")
	fmt.Fprintln(&script, "set -eu")
	if s.controllerUmask != 0 {
		fmt.Fprintf(&script, "umask %d\n", s.controllerUmask)
	}
	fmt.Fprintf(&script, "export ETCD_UNSUPPORTED_ARCH='%s'\n", runtime.GOARCH)
	fmt.Fprintf(&script, "%s controller --debug %s </dev/null >>/tmp/k0s-controller.log 2>&1 &\n", s.k0sFullPath, strings.Join(k0sArgs, " "))
	fmt.Fprintln(&script, "disown %1")

	if err := conn.Exec(ctx, "cat >/tmp/start-k0s && chmod +x /tmp/start-k0s", SSHStreams{
		In: strings.NewReader(script.String()),
	}); err != nil {
		return fmt.Errorf("failed to write start script: %w", err)
	}

	return s.StartController(ctx, conn)
}

// StartController starts a k0s controller that was initialized in "standalone" mode.
func (s *standaloneLaunchDelegate) StartController(ctx context.Context, conn *SSHConnection) error {
	const cmd = "/tmp/start-k0s"
	if err := conn.Exec(ctx, cmd, SSHStreams{}); err != nil {
		return fmt.Errorf("unable to execute %q: %w", cmd, err)
	}
	return nil
}

// StopController stops a k0s controller that was started in "standalone" mode.
func (s *standaloneLaunchDelegate) StopController(ctx context.Context, conn *SSHConnection) error {
	return s.killK0s(ctx, conn)
}

// InitWorker initializes a worker in "standalone" mode, meaning that the k0s
// executable is launched directly (vs. started via a service manager).
func (s *standaloneLaunchDelegate) InitWorker(ctx context.Context, conn *SSHConnection, token string, k0sArgs ...string) error {
	if token == "" {
		return fmt.Errorf("got empty token for worker join")
	}

	var script strings.Builder
	fmt.Fprintln(&script, "#!/usr/bin/env bash")
	fmt.Fprintln(&script, "set -eu")
	fmt.Fprintf(&script, "%s worker --debug %s \"$@\" </dev/null >>/tmp/k0s-worker.log 2>&1 &\n", s.k0sFullPath, strings.Join(k0sArgs, " "))
	fmt.Fprintln(&script, "disown %1")

	if err := conn.Exec(ctx, "cat >/tmp/start-k0s-worker && chmod +x /tmp/start-k0s-worker", SSHStreams{
		In: strings.NewReader(script.String()),
	}); err != nil {
		return fmt.Errorf("failed to write start script: %w", err)
	}

	if err := conn.Exec(ctx, "/tmp/start-k0s-worker "+token, SSHStreams{}); err != nil {
		return fmt.Errorf("unable to execute %q: %w", "/tmp/start-k0s-worker <token>", err)
	}
	return nil
}

// StartWorker starts a k0s worker that was initialized in "standalone" mode.
func (s *standaloneLaunchDelegate) StartWorker(ctx context.Context, conn *SSHConnection) error {
	const cmd = "/tmp/start-k0s-worker"
	if err := conn.Exec(ctx, cmd, SSHStreams{}); err != nil {
		return fmt.Errorf("unable to execute %q: %w", cmd, err)
	}
	return nil
}

// StopWorker stops a k0s worker that was started in "standalone" mode.
func (s *standaloneLaunchDelegate) StopWorker(ctx context.Context, conn *SSHConnection) error {
	return s.killK0s(ctx, conn)
}

func (s *standaloneLaunchDelegate) ReadK0sLogs(ctx context.Context, conn *SSHConnection, out, _ io.Writer) error {
	return conn.Exec(ctx, "cat /tmp/k0s-*.log", SSHStreams{Out: out})
}

func (s *standaloneLaunchDelegate) killK0s(ctx context.Context, conn *SSHConnection) error {
	stopCommand := fmt.Sprintf("kill $(pidof %s | tr \" \" \"\\n\" | sort -n | head -n1) && while pidof %s; do sleep 0.1s; done", s.k0sFullPath, s.k0sFullPath)
	if err := conn.Exec(ctx, stopCommand, SSHStreams{}); err != nil {
		return fmt.Errorf("unable to execute %q: %w", stopCommand, err)
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
	if err := conn.Exec(ctx, cmd, SSHStreams{}); err != nil {
		return fmt.Errorf("unable to execute %q: %w", cmd, err)
	}

	return o.StartController(ctx, conn)
}

// StartController starts a k0s controller that was started using OpenRC.
func (o *openRCLaunchDelegate) StartController(ctx context.Context, conn *SSHConnection) error {
	const cmd = "/etc/init.d/k0scontroller start"
	if err := conn.Exec(ctx, cmd, SSHStreams{}); err != nil {
		return fmt.Errorf("unable to execute %q: %w", cmd, err)
	}
	return nil
}

// StopController stops a k0s controller that was started using OpenRC.
func (*openRCLaunchDelegate) StopController(ctx context.Context, conn *SSHConnection) error {
	startCmd := "/etc/init.d/k0scontroller stop"
	if err := conn.Exec(ctx, startCmd, SSHStreams{}); err != nil {
		return fmt.Errorf("unable to execute %q: %w", startCmd, err)
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
	if err := conn.Exec(ctx, cmd, SSHStreams{}); err != nil {
		return fmt.Errorf("unable to execute %q: %w", cmd, err)
	}

	return nil
}

// StartWorker starts a k0s worker that was started using OpenRC.
func (o *openRCLaunchDelegate) StartWorker(ctx context.Context, conn *SSHConnection) error {
	const cmd = "/etc/init.d/k0sworker start"
	if err := conn.Exec(ctx, cmd, SSHStreams{}); err != nil {
		return fmt.Errorf("unable to execute %q: %w", cmd, err)
	}
	return nil
}

// StopWorker stops a k0s worker that was started using OpenRC.
func (*openRCLaunchDelegate) StopWorker(ctx context.Context, conn *SSHConnection) error {
	startCmd := "/etc/init.d/k0sworker stop"
	if err := conn.Exec(ctx, startCmd, SSHStreams{}); err != nil {
		return fmt.Errorf("unable to execute %q: %w", startCmd, err)
	}
	return nil
}

func (*openRCLaunchDelegate) ReadK0sLogs(ctx context.Context, conn *SSHConnection, out, err io.Writer) error {
	var wg sync.WaitGroup
	var outErr, errErr error

	wg.Add(1)
	go func() {
		defer wg.Done()
		outErr = conn.Exec(ctx, "cat /var/log/k0s.log", SSHStreams{Out: out})
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		errErr = conn.Exec(ctx, "cat /var/log/k0s.err", SSHStreams{Out: err})
	}()

	wg.Wait()

	return errors.Join(outErr, errErr)
}

// installK0sServiceOpenRC will install an OpenRC k0s-type service (controller/worker)
// if it does not already exist.
func (o *openRCLaunchDelegate) installK0sService(ctx context.Context, conn *SSHConnection, k0sType string) error {
	existsCommand := fmt.Sprintf("/usr/bin/file /etc/init.d/k0s%s", k0sType)
	if _, err := conn.ExecWithOutput(ctx, existsCommand); err != nil {
		cmd := fmt.Sprintf("%s install %s", o.k0sFullPath, k0sType)
		if err := conn.Exec(ctx, cmd, SSHStreams{}); err != nil {
			return fmt.Errorf("unable to execute %q: %w", cmd, err)
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
