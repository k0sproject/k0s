//go:build unix

// SPDX-FileCopyrightText: 2021 k0s authors
// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"context"
	"testing"
	"time"

	aptu "github.com/k0sproject/k0s/internal/autopilot/testutil"
	"github.com/k0sproject/k0s/internal/testutil"
	apcli "github.com/k0sproject/k0s/pkg/autopilot/client"
	aproot "github.com/k0sproject/k0s/pkg/autopilot/controller/root"
	"github.com/k0sproject/k0s/pkg/leaderelection"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"golang.org/x/sync/errgroup"
)

type fakeLeaderElector chan leaderelection.Status

// Run implements leaderElector.
func (le fakeLeaderElector) Run(ctx context.Context, callback func(leaderelection.Status)) {
	for {
		select {
		case <-ctx.Done():
			return
		case status, ok := <-le:
			if !ok {
				return
			}
			callback(status)
		}
	}
}

// TestModeSwitch tests the scenario of losing + re-acquiring the kubernetes lease.
// This toggle should result in sub-controllers being shutdown and then restarted.
func TestModeSwitch(t *testing.T) {
	logger := logrus.New().WithField("app", "autopilot-test")
	clientFactory := aptu.NewFakeClientFactory()

	rootControllerInterface, err := NewRootController(aproot.RootConfig{}, logger, false, testutil.NewFakeClientFactory(), clientFactory)
	assert.NoError(t, err)

	rootController, ok := rootControllerInterface.(*rootController)
	assert.True(t, ok)
	assert.NotEmpty(t, rootController)

	awaitFirstStart := make(chan struct{})
	var seenEvents []string

	// Override the important portions of leasewatcher, and provide wrappers to the start/stop
	// sub-controller handlers for invocation counting.
	leaseEventStatusCh := make(fakeLeaderElector)
	rootController.newLeaderElector = func(c leaderelection.Config) (leaderElector, error) {
		return &leaseEventStatusCh, nil
	}
	rootController.startSubHandler = func(ctx context.Context, event leaderelection.Status) (context.CancelFunc, *errgroup.Group) {
		if len(seenEvents) < 1 {
			close(awaitFirstStart)
		}
		seenEvents = append(seenEvents, "start: "+event.String())
		return rootController.startSubControllers(ctx, event)
	}
	rootController.startSubHandlerRoutine = func(ctx context.Context, logger *logrus.Entry, event leaderelection.Status) error {
		<-ctx.Done()
		return nil
	}
	rootController.stopSubHandler = func(cancel context.CancelFunc, g *errgroup.Group, event leaderelection.Status) {
		seenEvents = append(seenEvents, "stop: "+event.String())
		rootController.stopSubControllers(cancel, g, event)
	}
	rootController.setupHandler = func(ctx context.Context, cf apcli.FactoryInterface) error {
		return nil
	}

	ctx, cancel := context.WithCancel(t.Context())

	// Send alternating lease events, as well as one that is considered redundant

	go func() {
		logger.Info("Awaiting first start")
		<-awaitFirstStart

		logger.Info("Sending acquired")
		leaseEventStatusCh <- leaderelection.StatusLeading

		logger.Info("Sending acquired (again)")
		leaseEventStatusCh <- leaderelection.StatusLeading

		time.Sleep(1 * time.Second)
		logger.Info("Canceling context")
		cancel()
	}()

	assert.NoError(t, rootController.Run(ctx))

	assert.Equal(t, []string{
		// The controller will always start in pending state.
		"start: pending",

		// The first leading status is observed.
		"stop: leading",
		"start: leading",

		// The second leading status is ignored, as the controller is already in
		// the right state.

		// Finally, the context gets canceled and the controller shuts down.
		"stop: leading",
	}, seenEvents)
}
