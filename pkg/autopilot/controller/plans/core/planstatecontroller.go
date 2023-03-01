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

package core

import (
	"context"
	"fmt"
	"time"

	apv1beta2 "github.com/k0sproject/k0s/pkg/apis/autopilot/v1beta2"

	"github.com/sirupsen/logrus"
	cr "sigs.k8s.io/controller-runtime"
	crcli "sigs.k8s.io/controller-runtime/pkg/client"
	crrec "sigs.k8s.io/controller-runtime/pkg/reconcile"
)

const (
	defaultRequeueDuration = 5 * time.Second
)

type planStateController struct {
	name    string
	logger  *logrus.Entry
	client  crcli.Client
	handler PlanStateHandler
}

// NewPlanStateController creates a new `PlanStateController` with parameterized handler
// for specialized reconciliation processing.
func NewPlanStateController(name string, logger *logrus.Entry, client crcli.Client, handler PlanStateHandler) crrec.Reconciler {
	return &planStateController{
		name:    name,
		logger:  logger,
		client:  client,
		handler: handler,
	}
}

// Reconcile performs the basic operations for every controller: obtain the plan, copy it,
// delgate to the handler, and update status if available.
func (c *planStateController) Reconcile(ctx context.Context, req cr.Request) (cr.Result, error) {
	logger := c.logger.WithField("controller", c.name)

	plan := apv1beta2.Plan{}
	if err := c.client.Get(ctx, req.NamespacedName, &plan); err != nil {
		logger.Warnf("Unable to get plan for request '%s': %v", req.NamespacedName, err)
		return cr.Result{}, nil
	}

	planCopy := plan.DeepCopy()

	// Pass the processing along to the `PlanStateHandler`
	res, err := c.handler.Handle(ctx, planCopy)
	if err != nil {
		// Don't send errors back to controller-runtime, as that will cause the event to get requeued
		// which we don't want. Requeuing is explicit on our side.

		logger.Errorf("Unable to process state controller handler: %v", err)
		return cr.Result{}, nil
	}

	if res == ProviderResultRetry {
		c.logger.Info("Requeuing request due to explicit retry")
		return cr.Result{RequeueAfter: defaultRequeueDuration}, nil
	}

	if err := c.client.Status().Update(ctx, planCopy, &crcli.SubResourceUpdateOptions{}); err != nil {
		return cr.Result{}, fmt.Errorf("unable to update plan '%s' with status: %w", req.NamespacedName, err)
	}

	return cr.Result{}, nil
}
