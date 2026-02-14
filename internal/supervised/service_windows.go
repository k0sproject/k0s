// SPDX-FileCopyrightText: 2025 k0s authors
// SPDX-License-Identifier: Apache-2.0

package supervised

import (
	"cmp"
	"context"
	"errors"
	"flag"
	"os"
	"slices"
	"strings"
	"sync"

	"fmt"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"golang.org/x/sys/windows/svc"
	"golang.org/x/sys/windows/svc/eventlog"
)

type handlerFunc func(requests <-chan svc.ChangeRequest, updates chan<- svc.Status) (ssec bool, errno uint32)

// Execute implements [svc.Handler].
func (h handlerFunc) Execute(_ []string, requests <-chan svc.ChangeRequest, updates chan<- svc.Status) (ssec bool, errno uint32) {
	return h(requests, updates)
}

func runService(ctx context.Context, main *cobra.Command) (err error) {
	serviceName, err := popServiceName()
	if err != nil {
		return err
	}

	return svc.Run(serviceName, handlerFunc(func(requests <-chan svc.ChangeRequest, updates chan<- svc.Status) (ssec bool, errno uint32) {
		return serviceMain(ctx, serviceName, main, requests, updates)
	}))
}

func popServiceName() (string, error) {
	if len(os.Args) > 1 {
		if serviceName, found := strings.CutPrefix(os.Args[1], "service="); found {
			if serviceName == "" {
				return "", errors.New("service name is empty")
			}

			os.Args = slices.Delete(os.Args, 1, 2)
			return serviceName, nil
		}
	}

	return "", errors.New("failed to determine service name")
}

type serviceConn struct {
	log      EventLog
	requests <-chan svc.ChangeRequest
	updates  chan<- svc.Status
}

func serviceMain(ctx context.Context, serviceName string, main *cobra.Command, requests <-chan svc.ChangeRequest, updates chan<- svc.Status) (ssec bool, errno uint32) {
	elog := eventLog{logrus.WithFields(logrus.Fields{
		"serviceName": serviceName,
		"component":   "eventlog",
	}), nil}

	if log, err := eventlog.Open(serviceName); err != nil {
		logrus.WithError(err).Error("Failed to open Windows event log")
	} else {
		elog.elog = log
		defer log.Close()
	}

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	conn := serviceConn{&elog, requests, updates}
	err := conn.serve(func() (serviceStateFunc, error) {
		return conn.serviceStartPending(ctx, main)
	})
	if err != nil {
		logrus.WithError(err).Error("Service failed")
		elog.Error(1, "Service failed: ", err)
		return true, 2
	}

	logrus.Info("Service stopped")
	elog.Info(2, "Service stopped: ", err)
	return false, 0
}

type markReady func()

func (m markReady) MarkReady() { m() }

// https://learn.microsoft.com/en-us/windows/win32/api/winsvc/ns-winsvc-service_status
const acceptsStop = svc.AcceptStop | svc.AcceptPreShutdown

type serviceStateFunc func() (serviceStateFunc, error)

func (c serviceConn) serve(initialState serviceStateFunc) error {
	for currentState := initialState; currentState != nil; {
		var err error
		if currentState, err = currentState(); err != nil {
			return err
		}
	}

	return nil
}

func (c *serviceConn) serviceStartPending(ctx context.Context, main *cobra.Command) (_ serviceStateFunc, err error) {
	c.updates <- svc.Status{State: svc.StartPending, Accepts: acceptsStop}

	ctx, cancel := context.WithCancelCause(ctx)
	defer func() {
		if cancel != nil {
			cancel(err)
		}
	}()

	ready := make(chan struct{})
	stopped := make(chan error, 1)

	go func() {
		defer close(stopped)
		supervised := markReady(sync.OnceFunc(func() { close(ready) }))
		ctx := set(ctx, supervised)

		// Not running interactively. Suppress writing to stdout/stderr.
		var helpErr error
		main.SilenceErrors = true
		main.SilenceUsage = true
		main.SetHelpFunc(func(*cobra.Command, []string) {
			helpErr = flag.ErrHelp
		})

		err := main.ExecuteContext(ctx)
		stopped <- cmp.Or(err, helpErr)
	}()

	for {
		select {
		case <-ready:
			cancelRunning := cancel
			cancel = nil
			return c.serviceRunning(cancelRunning, stopped), nil

		case request := <-c.requests:
			if err := c.handleStopRequest(request); err != nil {
				cancel(err)
				return c.serviceStopPending(stopped), nil
			}

		case err := <-stopped:
			cancel(err)
			return nil, err
		}
	}
}

func (c *serviceConn) serviceRunning(cancel context.CancelCauseFunc, stopped <-chan error) serviceStateFunc {
	c.updates <- svc.Status{State: svc.Running, Accepts: acceptsStop}

	return func() (_ serviceStateFunc, err error) {
		defer cancel(err)

		for {
			select {
			case request := <-c.requests:
				if err := c.handleStopRequest(request); err != nil {
					cancel(err)
					return c.serviceStopPending(stopped), nil
				}

			case err := <-stopped:
				return nil, err
			}
		}
	}
}

func (c *serviceConn) serviceStopPending(stopped <-chan error) serviceStateFunc {
	c.updates <- svc.Status{State: svc.StopPending}

	return func() (serviceStateFunc, error) {
		for {
			select {
			case request := <-c.requests:
				c.handleOther(request)

			case err := <-stopped:
				return nil, err
			}
		}
	}
}

func (c *serviceConn) handleStopRequest(request svc.ChangeRequest) error {
	switch request.Cmd {
	case svc.Stop:
		return errors.New("service stop request received")
	case svc.PreShutdown:
		return errors.New("system shutdown request received")
	default:
		c.handleOther(request)
		return nil
	}
}

func (c *serviceConn) handleOther(request svc.ChangeRequest) {
	if request.Cmd == svc.Interrogate {
		c.updates <- request.CurrentStatus
		return
	}

	c.log.Error(3, fmt.Sprintf("unexpected service control request %d", request.Cmd))
}
