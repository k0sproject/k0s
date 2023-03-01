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

package delegate

import (
	apv1beta2 "github.com/k0sproject/k0s/pkg/apis/autopilot/v1beta2"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	crcli "sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	ControllerDelegateWorker     = "worker"
	ControllerDelegateController = "controller"
)

func NewControllerDelegateMap() ControllerDelegateMap {
	return map[string]ControllerDelegate{
		ControllerDelegateWorker:     NodeControllerDelegate(),
		ControllerDelegateController: ControlNodeControllerDelegate(),
	}
}

type ControllerDelegateOption func(delegate controllerDelegate) controllerDelegate

// WithReadyForUpdateFunc assigns a new function for handling the testing of
// if a delegate implementation is ready for a signaling update.
func WithReadyForUpdateFunc(f k0sUpdateReadyFunc) ControllerDelegateOption {
	return func(delegate controllerDelegate) controllerDelegate {
		delegate.k0sUpdateReady = f
		return delegate
	}
}

// ControlNodeControllerDelegate creates a `ControllerDelegate` for use with
// `apv1beta2.ControlNode` objects
func ControlNodeControllerDelegate(opts ...ControllerDelegateOption) ControllerDelegate {
	delegate := controllerDelegate{
		"ControlNode",
		func() crcli.Object {
			return &apv1beta2.ControlNode{}
		},
		func() crcli.ObjectList {
			return &apv1beta2.ControlNodeList{}
		},
		func(status apv1beta2.PlanCommandK0sUpdateStatus, obj crcli.Object) K0sUpdateReadyStatus {
			return CanUpdate
		},
		func(list crcli.ObjectList, status apv1beta2.PlanCommandTargetStateType) []apv1beta2.PlanCommandTargetStatus {
			nodes := make([]apv1beta2.PlanCommandTargetStatus, 0)
			if cnl, ok := list.(*apv1beta2.ControlNodeList); ok {
				for _, item := range cnl.Items {
					nodes = append(nodes, apv1beta2.PlanCommandTargetStatus{
						Name:                 item.GetName(),
						State:                status,
						LastUpdatedTimestamp: metav1.Now(),
					})
				}
			}

			return nodes
		},
		func(name string) types.NamespacedName {
			return types.NamespacedName{Name: name}
		},
		func(o crcli.Object) crcli.Object {
			if obj, ok := o.(*apv1beta2.ControlNode); ok {
				return obj.DeepCopy()
			}

			return nil
		},
	}

	for _, opt := range opts {
		delegate = opt(delegate)
	}

	return delegate
}

// NodeControllerDelegate creates a `ControllerDelegate` for use with `v1.Node` objects
func NodeControllerDelegate(opts ...ControllerDelegateOption) ControllerDelegate {
	delegate := controllerDelegate{
		"Node",
		func() crcli.Object {
			return &v1.Node{}
		},
		func() crcli.ObjectList {
			return &v1.NodeList{}
		},
		func(status apv1beta2.PlanCommandK0sUpdateStatus, obj crcli.Object) K0sUpdateReadyStatus {
			if node, ok := obj.(*v1.Node); ok {
				for _, cond := range node.Status.Conditions {
					if cond.Type == v1.NodeReady && cond.Status == v1.ConditionTrue {
						return CanUpdate
					}
				}
			}

			return NotReady
		},
		func(list crcli.ObjectList, status apv1beta2.PlanCommandTargetStateType) []apv1beta2.PlanCommandTargetStatus {
			nodes := make([]apv1beta2.PlanCommandTargetStatus, 0)
			if nl, ok := list.(*v1.NodeList); ok {
				for _, item := range nl.Items {
					nodes = append(nodes, apv1beta2.PlanCommandTargetStatus{
						Name:                 item.GetName(),
						State:                status,
						LastUpdatedTimestamp: metav1.Now(),
					})
				}
			}

			return nodes
		},
		func(name string) types.NamespacedName {
			return types.NamespacedName{Name: name}
		},
		func(o crcli.Object) crcli.Object {
			if obj, ok := o.(*v1.Node); ok {
				return obj.DeepCopy()
			}

			return nil
		},
	}

	for _, opt := range opts {
		delegate = opt(delegate)
	}

	return delegate
}
