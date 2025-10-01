// SPDX-FileCopyrightText: 2023 k0s authors
// SPDX-License-Identifier: Apache-2.0

package autopilot

import (
	"context"
	"fmt"

	apv1beta2 "github.com/k0sproject/k0s/pkg/apis/autopilot/v1beta2"
	appc "github.com/k0sproject/k0s/pkg/autopilot/controller/plans/core"
	apclient "github.com/k0sproject/k0s/pkg/client/clientset"
	"github.com/k0sproject/k0s/pkg/kubernetes/watch"

	"github.com/k0sproject/k0s/inttest/common"

	extensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	extensionsclient "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset/typed/apiextensions/v1"

	"github.com/sirupsen/logrus"
)

// WaitForPlanState waits for the Plan with the given name to reach the given state.
func WaitForPlanState(ctx context.Context, client apclient.Interface, name string, state apv1beta2.PlanStateType) (plan *apv1beta2.Plan, err error) {
	err = watch.FromClient[*apv1beta2.PlanList, apv1beta2.Plan](client.AutopilotV1beta2().Plans()).
		WithObjectName(name).
		WithErrorCallback(common.RetryWatchErrors(logrus.Infof)).
		Until(ctx, func(candidate *apv1beta2.Plan) (bool, error) {
			switch candidate.Status.State {
			case state:
				plan = candidate
				return true, nil

			// Those are non-terminal states, so keep on waiting.
			case appc.PlanSchedulable, appc.PlanSchedulableWait, "":
				return false, nil

			// All other states are considered terminal.
			default:
				return false, fmt.Errorf("unexpected Plan state: %s", candidate.Status.State)
			}
		})
	return
}

// WaitForCRDByName waits until the CRD with the given name is established.
// The given name is suffixed with the autopilot's API group.
func WaitForCRDByName(ctx context.Context, client extensionsclient.ApiextensionsV1Interface, name string) error {
	return WaitForCRDByGroupName(ctx, client, fmt.Sprintf("%s.%s", name, apv1beta2.GroupName))
}

// WaitForCRDByGroupName waits until the CRD with the given name is established.
func WaitForCRDByGroupName(ctx context.Context, client extensionsclient.ApiextensionsV1Interface, name string) error {
	return watch.CRDs(client.CustomResourceDefinitions()).
		WithObjectName(name).
		WithErrorCallback(common.RetryWatchErrors(logrus.Infof)).
		Until(ctx, func(item *extensionsv1.CustomResourceDefinition) (bool, error) {
			for _, cond := range item.Status.Conditions {
				if cond.Type == extensionsv1.Established {
					return cond.Status == extensionsv1.ConditionTrue, nil
				}
			}

			return false, nil
		})
}
