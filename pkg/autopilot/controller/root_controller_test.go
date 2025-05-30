//go:build unix

// Copyright 2021 k0s authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

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

type fakeLeaseWatcher struct {
	leaseEventStatusCh chan leaderelection.Status
	errorsCh           chan error
}

var _ LeaseWatcher = (*fakeLeaseWatcher)(nil)

// NewFakeLeaseWatcher creates a LeaseWatcher where the source channel
// is made available, for simulating lease changes.
func NewFakeLeaseWatcher() (LeaseWatcher, chan leaderelection.Status) {
	leaseEventStatusCh := make(chan leaderelection.Status, 10)
	errorsCh := make(chan error, 10)

	return &fakeLeaseWatcher{
		leaseEventStatusCh: leaseEventStatusCh,
		errorsCh:           errorsCh,
	}, leaseEventStatusCh
}

// StartWatcher for the fake LeaseWatcher just propagates the premade lease event channel
func (lw *fakeLeaseWatcher) StartWatcher(ctx context.Context, namespace string, name, identity string) (<-chan leaderelection.Status, <-chan error) {
	return lw.leaseEventStatusCh, lw.errorsCh
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

	var seenEvents []string

	// Override the important portions of leasewatcher, and provide wrappers to the start/stop
	// sub-controller handlers for invocation counting.
	leaseWatcher, leaseEventStatusCh := NewFakeLeaseWatcher()
	rootController.leaseWatcherCreator = func(e *logrus.Entry, cf apcli.FactoryInterface) (LeaseWatcher, error) {
		return leaseWatcher, nil
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

	ctx, cancel := context.WithCancel(t.Context())

	// Send alternating lease events, as well as one that is considered redundant

	go func() {
		logger.Info("Sending pending")
		leaseEventStatusCh <- leaderelection.StatusPending

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
