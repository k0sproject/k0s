// SPDX-FileCopyrightText: 2026 k0s authors
// SPDX-License-Identifier: Apache-2.0

package helm

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/cli-runtime/pkg/genericclioptions"

	"github.com/sirupsen/logrus"
	"helm.sh/helm/v3/pkg/kube"
)

// A bespoke Helm kube client to make readiness waits context-aware.
//
// The embedded kube.Client is still used for regular Helm kube operations; only
// wait behavior is overridden.
type client struct {
	*kube.Client
	ctx context.Context
	log logrus.FieldLogger
}

// Creates a Helm kube client that binds readiness waits to ctx so they can be
// interrupted by the surrounding Helm action lifecycle.
func newKubeClient(ctx context.Context, getter genericclioptions.RESTClientGetter, log logrus.FieldLogger) *client {
	wrapped := kube.New(getter)
	wrapped.Log = log.WithField("client", "wrapped").Debugf
	return &client{wrapped, ctx, log}
}

// Wait overrides [kube.Client.Wait].
//
// Unlike Helm's default implementation, this implementation derives its timeout
// context from the client's context. This allows external interruptions to stop
// the wait before the retry timeout occurs.
func (c *client) Wait(resources kube.ResourceList, timeout time.Duration) error {
	c.log.Debug("Beginning to wait for ", len(resources), " resources with timeout of ", timeout)
	return c.waitForResources(resources, timeout, kube.PausedAsReady(true))
}

// WaitWithJobs overrides [kube.Client.WaitWithJobs].
//
// Unlike Helm's default implementation, this implementation derives its timeout
// context from the client's context. This allows external interruptions to stop
// the wait before the retry timeout occurs.
func (c *client) WaitWithJobs(resources kube.ResourceList, timeout time.Duration) error {
	c.log.Debug("Beginning to wait for ", len(resources), " resources with timeout of ", timeout, " including jobs")
	return c.waitForResources(resources, timeout, kube.PausedAsReady(true), kube.CheckJobs(true))
}

// WaitForDelete overrides [kube.Client.WaitForDelete].
//
// Unlike Helm's default implementation, this implementation derives its timeout
// context from the client's context. This allows external interruptions to stop
// the wait before the retry timeout occurs.
func (c *client) WaitForDelete(resources kube.ResourceList, timeout time.Duration) error {
	c.log.Debug("Beginning to wait for ", len(resources), " resources to be deleted with timeout of ", timeout)

	ctx, cancel := context.WithTimeout(c.ctx, timeout)
	defer cancel()

	timer := time.NewTimer(0)
	defer timer.Stop()

	for {
		var allDeleted bool
		done := make(chan struct{})
		go func() {
			defer close(done)
			for _, v := range resources {
				select {
				case <-ctx.Done():
					return
				default:
					if !apierrors.IsNotFound(v.Get()) {
						return
					}
				}
			}
			allDeleted = true
		}()

		select {
		case <-done:
			if allDeleted {
				return nil
			}
		case <-ctx.Done():
			return context.Cause(ctx)
		}

		timer.Reset(2 * time.Second)
		select {
		case <-timer.C:
		case <-ctx.Done():
			return context.Cause(ctx)
		}
	}
}

// Polls all resources until each becomes ready, the client's context gets
// canceled, the timeout expires, or a terminal API error is observed.
func (c *client) waitForResources(resources kube.ResourceList, timeout time.Duration, opts ...kube.ReadyCheckerOption) error {
	clients, err := c.Factory.KubernetesClientSet()
	if err != nil {
		return err
	}

	checker := kube.NewReadyChecker(clients, c.log.WithField("client", "readychecker").Debugf, opts...)

	ctx, cancel := context.WithTimeout(c.ctx, timeout)
	defer cancel()

	timer := time.NewTimer(0)
	defer timer.Stop()

checkAllResources:
	for {
		for _, resource := range resources {
			if ready, err := checker.IsReady(ctx, resource); err != nil {
				var stop bool
				select {
				case <-ctx.Done():
					stop = true
				default:
					var status apierrors.APIStatus
					if errors.As(err, &status) {
						code := status.Status().Code
						stop = code != 0 && code != http.StatusTooManyRequests && (code < 500 || code == http.StatusNotImplemented)
					}
				}

				resourceDesc := resource.ObjectName()
				if resource.Namespaced() {
					resourceDesc += " in namespace " + resource.Namespace
				}

				if stop {
					return fmt.Errorf("while checking for readiness of %s: %w", resourceDesc, err)
				}

				c.log.WithError(err).Debug("While checking for readiness of ", resourceDesc)
			} else if ready {
				continue
			}

			timer.Reset(2 * time.Second)

			select {
			case <-timer.C:
				continue checkAllResources
			case <-ctx.Done():
				return context.Cause(ctx)
			}
		}

		return nil
	}
}
