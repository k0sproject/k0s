// SPDX-FileCopyrightText: 2020 k0s authors
// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"context"
	"errors"
	"time"

	"github.com/k0sproject/k0s/pkg/applier"
	"github.com/k0sproject/k0s/pkg/component/controller/leaderelector"
	"github.com/k0sproject/k0s/pkg/component/manager"
	"github.com/k0sproject/k0s/pkg/kubernetes"
	"github.com/k0sproject/k0s/pkg/leaderelection"
	"github.com/k0sproject/k0s/static"
	"github.com/sirupsen/logrus"
)

// CRDStack applies bundled CRDs.
type CRDStack struct {
	clients       kubernetes.ClientFactoryInterface
	leaderElector leaderelector.Interface
	bundle        string
	stop          func()

	crdOpts
}

type crdOpts struct {
	stackName, assetsDir string
}

type CRDOption func(*crdOpts)

var _ manager.Component = (*CRDStack)(nil)

// Creates a new CRD stack for the given bundle.
func NewCRDStack(clients kubernetes.ClientFactoryInterface, leaderElector leaderelector.Interface, bundle string, opts ...CRDOption) *CRDStack {
	var options crdOpts
	for _, opt := range opts {
		opt(&options)
	}

	if options.assetsDir == "" {
		options.stackName = bundle
		options.assetsDir = bundle
	}

	return &CRDStack{
		clients:       clients,
		leaderElector: leaderElector,
		bundle:        bundle,
		crdOpts:       options,
	}
}

func WithStackName(stackName string) CRDOption {
	return func(opts *crdOpts) { opts.stackName = stackName }
}

func WithCRDAssetsDir(assetsDir string) CRDOption {
	return func(opts *crdOpts) { opts.assetsDir = assetsDir }
}

// Init implements [manager.Component]. It does nothing.
func (c *CRDStack) Init(ctx context.Context) error {
	return nil
}

// Applies this CRD stack when becoming the leader. Implements [manager.Component].
func (c *CRDStack) Start(context.Context) error {
	ctx, cancel := context.WithCancelCause(context.Background())
	done := make(chan struct{})

	go func() {
		defer close(done)
		leaderelection.RunLeaderTasks(ctx, c.leaderElector.CurrentStatus, func(ctx context.Context) {
			resources, err := applier.ReadUnstructuredDir(static.CRDs, c.assetsDir)
			if err != nil {
				logrus.WithError(err).Errorf("Failed to read %s CRD stack", c.bundle)
				return
			}

			stack := applier.Stack{
				Name:      c.stackName,
				Resources: resources,
				Clients:   c.clients,
			}

			for {
				err := stack.Apply(ctx, true)
				if err == nil {
					break
				}

				logrus.WithError(err).Errorf("Failed to apply %s CRD stack, retrying in 30 seconds", c.bundle)

				select {
				case <-time.After(30 * time.Second):
				case <-ctx.Done():
					return
				}
			}
		})
	}()

	c.stop = func() { cancel(errors.New("CRD stack is stopping")); <-done }

	return nil
}

// Stop implements [manager.Component].
func (c *CRDStack) Stop() error {
	if stop := c.stop; stop != nil {
		stop()
	}
	return nil
}
