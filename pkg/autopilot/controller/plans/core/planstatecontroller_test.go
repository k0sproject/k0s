// SPDX-FileCopyrightText: 2021 k0s authors
// SPDX-License-Identifier: Apache-2.0

package core

import (
	"context"
	"testing"

	apv1beta2 "github.com/k0sproject/k0s/pkg/apis/autopilot/v1beta2"
	apscheme2 "github.com/k0sproject/k0s/pkg/client/clientset/scheme"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	cr "sigs.k8s.io/controller-runtime"
	crcli "sigs.k8s.io/controller-runtime/pkg/client"
	crfake "sigs.k8s.io/controller-runtime/pkg/client/fake"
)

// fakePlanStateHandler provides a functional adaptation for `PlanStateHandler`
type fakePlanStateHandler struct {
	handle func(ctx context.Context, plan *apv1beta2.Plan) (ProviderResult, error)
}

// Handle delegates to our provided test handler function.
func (h *fakePlanStateHandler) Handle(ctx context.Context, plan *apv1beta2.Plan) (ProviderResult, error) {
	return h.handle(ctx, plan)
}

// TestReconcile runs a number of tests against the main reconciliation function of `PlanStateController`
// ensuring valid functionality and edge-cases.
func TestReconcile(t *testing.T) {
	var tests = []struct {
		name            string
		handler         PlanStateHandler
		plan            *apv1beta2.Plan
		expectedRequeue bool
		expectedStatus  *apv1beta2.PlanStatus
	}{
		// If a plan is not found, this shouldn't result in an error
		{
			"PlanNotFound",
			nil,
			&apv1beta2.Plan{},
			false,
			nil,
		},

		// The scenario where the handler returns an error. We don't want general applicatione errors making
		// their way back to controller-runtime (unless via explicit 'retry'), so we expect the handler to
		// log and 'nil' the error upwards.
		{
			"HandlerError",
			&fakePlanStateHandler{
				func(ctx context.Context, plan *apv1beta2.Plan) (ProviderResult, error) {
					return ProviderResultFailure, assert.AnError
				},
			},
			&apv1beta2.Plan{
				ObjectMeta: metav1.ObjectMeta{
					Name: "HandlerError",
				},
			},
			false,
			nil,
		},

		// When the handler returns a 'retry', we should see this in the controller-runtime 'Result'
		{
			"HandleRetry",
			&fakePlanStateHandler{
				func(ctx context.Context, plan *apv1beta2.Plan) (ProviderResult, error) {
					return ProviderResultRetry, nil
				},
			},
			&apv1beta2.Plan{
				ObjectMeta: metav1.ObjectMeta{
					Name: "HandleRetry",
				},
			},
			true,
			nil,
		},

		// Ensure that if the plan status is updated, the controller will update the plan
		// in controller-runtime.
		{
			"StatusUpdated",
			&fakePlanStateHandler{
				func(ctx context.Context, plan *apv1beta2.Plan) (ProviderResult, error) {
					plan.Status.State = PlanCompleted
					return ProviderResultSuccess, nil
				},
			},
			&apv1beta2.Plan{
				ObjectMeta: metav1.ObjectMeta{
					Name: "StatusUpdated",
				},
			},
			false,
			&apv1beta2.PlanStatus{
				State: PlanCompleted,
			},
		},
	}

	scheme := runtime.NewScheme()
	assert.NoError(t, apscheme2.AddToScheme(scheme))
	assert.NoError(t, v1.AddToScheme(scheme))

	for _, test := range tests {
		objs := []crcli.Object{test.plan}
		client := crfake.NewClientBuilder().
			WithScheme(scheme).
			WithObjects(objs...).
			WithStatusSubresource(objs...).
			Build()

		t.Run(test.name, func(t *testing.T) {
			controller := NewPlanStateController(test.name, logrus.NewEntry(logrus.StandardLogger()), client, test.handler)
			req := cr.Request{NamespacedName: types.NamespacedName{Name: test.name}}

			ctx := t.Context()
			res, err := controller.Reconcile(ctx, req)
			assert.NoError(t, err)
			assert.Equal(t, test.expectedRequeue, !res.IsZero())

			if test.expectedStatus != nil {
				updatedPlan := apv1beta2.Plan{}

				assert.NoError(t, client.Get(ctx, req.NamespacedName, &updatedPlan))
				assert.Equal(t, *test.expectedStatus, updatedPlan.Status)
			}
		})
	}
}
