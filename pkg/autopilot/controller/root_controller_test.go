//go:build unix

// SPDX-FileCopyrightText: 2021 k0s authors
// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"context"
	"sync"
	"testing"
	"testing/synctest"

	aptu "github.com/k0sproject/k0s/internal/autopilot/testutil"
	apcli "github.com/k0sproject/k0s/pkg/autopilot/client"
	aproot "github.com/k0sproject/k0s/pkg/autopilot/controller/root"
	"github.com/k0sproject/k0s/pkg/leaderelection"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
	synctest.Test(t, func(t *testing.T) {
		logger := logrus.New().WithField("app", "autopilot-test")
		clientFactory := aptu.NewFakeClientFactory()

		rootControllerInterface, err := NewRootController(aproot.RootConfig{}, logger, false, clientFactory.Unwrap(), clientFactory)
		assert.NoError(t, err)

		rootController, ok := rootControllerInterface.(*rootController)
		assert.True(t, ok)
		assert.NotEmpty(t, rootController)

		var seenEvents []string

		// Override the important portions of leasewatcher, and provide wrappers to the start/stop
		// sub-controller handlers for invocation counting.
		leaseEventStatusCh := make(fakeLeaderElector)
		rootController.newLeaderElector = func(c leaderelection.Config) (leaderElector, error) {
			return &leaseEventStatusCh, nil
		}
		rootController.startSubHandler = func(ctx context.Context, event leaderelection.Status) (context.CancelFunc, *errgroup.Group) {
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

		var wg sync.WaitGroup
		ctx, cancel := context.WithCancel(t.Context())
		t.Cleanup(wg.Wait)
		t.Cleanup(cancel)
		wg.Go(func() { assert.NoError(t, rootController.Run(ctx)) })

		// Send alternating lease events, as well as one that is considered redundant

		// The controller will always start in pending state.
		logger.Info("Awaiting first start")
		synctest.Wait()
		require.Equal(t, []string{"start: pending"}, seenEvents)
		seenEvents = seenEvents[0:0]

		// The first leading status is observed.
		logger.Info("Sending acquired")
		leaseEventStatusCh <- leaderelection.StatusLeading
		synctest.Wait()
		require.Equal(t, []string{"stop: leading", "start: leading"}, seenEvents)
		seenEvents = seenEvents[0:0]

		// The second leading status is ignored, as the controller is already in
		// the right state.
		logger.Info("Sending acquired (again)")
		leaseEventStatusCh <- leaderelection.StatusLeading
		synctest.Wait()
		require.Empty(t, seenEvents)

		// Finally, the context gets canceled and the controller shuts down.
		logger.Info("Canceling context")
		cancel()
		synctest.Wait()
		require.Equal(t, []string{"stop: leading"}, seenEvents)
	})
}
