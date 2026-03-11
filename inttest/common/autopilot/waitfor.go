// SPDX-FileCopyrightText: 2023 k0s authors
// SPDX-License-Identifier: Apache-2.0

package autopilot

import (
	"context"
	"encoding/json"
	"fmt"

	apv1beta2 "github.com/k0sproject/k0s/pkg/apis/autopilot/v1beta2"
	apdel "github.com/k0sproject/k0s/pkg/autopilot/controller/delegate"
	appc "github.com/k0sproject/k0s/pkg/autopilot/controller/plans/core"
	apclient "github.com/k0sproject/k0s/pkg/client/clientset"
	"github.com/k0sproject/k0s/pkg/kubernetes/watch"

	"github.com/k0sproject/k0s/inttest/common"

	corev1 "k8s.io/api/core/v1"
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

// WaitForControlNodeReady watches the named ControlNode until it exists and has
// its platform labels (kubernetes.io/os and kubernetes.io/arch) set, indicating
// the autopilot controller has fully registered it.
func WaitForControlNodeReady(ctx context.Context, client apclient.Interface, name string) (*apv1beta2.ControlNode, error) {
	var cn *apv1beta2.ControlNode
	err := watch.ControlNodes(client.AutopilotV1beta2().ControlNodes()).
		WithObjectName(name).
		WithErrorCallback(common.RetryWatchErrors(logrus.Infof)).
		Until(ctx, func(candidate *apv1beta2.ControlNode) (bool, error) {
			labels := candidate.GetLabels()
			if labels[corev1.LabelOSStable] != "" && labels[corev1.LabelArchStable] != "" {
				cn = candidate
				return true, nil
			}
			return false, nil
		})
	return cn, err
}

// WaitForControlNodeSignalError watches the named ControlNode until the
// k0sproject.io/autopilot-last-error annotation is present and returns the
// parsed error, or the context times out.
func WaitForControlNodeSignalError(ctx context.Context, client apclient.Interface, name string) (*apdel.SignalError, error) {
	var signalError *apdel.SignalError
	err := watch.ControlNodes(client.AutopilotV1beta2().ControlNodes()).
		WithObjectName(name).
		WithErrorCallback(common.RetryWatchErrors(logrus.Infof)).
		Until(ctx, func(cn *apv1beta2.ControlNode) (bool, error) {
			raw, ok := cn.GetAnnotations()[apdel.SignalErrorAnnotation]
			if !ok {
				return false, nil
			}
			var e apdel.SignalError
			if err := json.Unmarshal([]byte(raw), &e); err != nil {
				return false, nil
			}
			signalError = &e
			return true, nil
		})
	return signalError, err
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
