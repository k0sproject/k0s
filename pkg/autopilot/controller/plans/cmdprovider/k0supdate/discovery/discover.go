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

package discovery

import (
	"context"

	apv1beta2 "github.com/k0sproject/k0s/pkg/apis/autopilot/v1beta2"
	apdel "github.com/k0sproject/k0s/pkg/autopilot/controller/delegate"
	appc "github.com/k0sproject/k0s/pkg/autopilot/controller/plans/core"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"
	crcli "sigs.k8s.io/controller-runtime/pkg/client"
)

type SignalObjectExistsFunc func(name string) (bool, *apv1beta2.PlanCommandTargetStateType)

// DiscoverNodes will find all of the `PlanCommandTarget` instances for the provided `PlanCommandTarget`,
// using the appropriate discovery method. If any nodes defined in the target don't exist,
// then `false` will be returned with the slice of `PlanCommandTarget` instances.
func DiscoverNodes(ctx context.Context, client crcli.Client, target *apv1beta2.PlanCommandTarget, delegate apdel.ControllerDelegate, exists SignalObjectExistsFunc) ([]apv1beta2.PlanCommandTargetStatus, bool) {
	discover := createDiscoverNodes(ctx, client, delegate, target)
	if discover == nil {
		// If we can't discover, assume that we're done with no results.
		return []apv1beta2.PlanCommandTargetStatus{}, true
	}

	return ensureNodesExist(discover(), exists)
}

// createDiscoverNodes returns a function that can be used to discover all nodes for
// the provided discovery type. If a discovery type cannot be found, nil is returned.
func createDiscoverNodes(ctx context.Context, client crcli.Client, delegate apdel.ControllerDelegate, target *apv1beta2.PlanCommandTarget) func() []apv1beta2.PlanCommandTargetStatus {
	if target.Discovery.Static != nil {
		return func() []apv1beta2.PlanCommandTargetStatus {
			return discoverNodesStatic(target)
		}
	}

	if target.Discovery.Selector != nil {
		return func() []apv1beta2.PlanCommandTargetStatus {
			return discoverNodesSelector(ctx, client, delegate, target)
		}
	}

	return nil
}

// discoverNodesStatic creates a slice of `PlanCommandTarget` instances found using the 'static'
// discovery method.
func discoverNodesStatic(target *apv1beta2.PlanCommandTarget) []apv1beta2.PlanCommandTargetStatus {
	nodes := make([]apv1beta2.PlanCommandTargetStatus, 0)

	if target.Discovery.Static != nil {
		for _, nodeName := range target.Discovery.Static.Nodes {
			nodes = append(nodes, apv1beta2.PlanCommandTargetStatus{Name: nodeName, State: appc.SignalPending, LastUpdatedTimestamp: metav1.Now()})
		}
	}

	return nodes
}

// discoverNodesSelector creates a slice of `PlanCommandTarget` instances found using the
// 'selector' discovery method.
func discoverNodesSelector(ctx context.Context, client crcli.Client, delegate apdel.ControllerDelegate, target *apv1beta2.PlanCommandTarget) []apv1beta2.PlanCommandTargetStatus {
	if target.Discovery.Selector != nil {
		opts := &crcli.ListOptions{}
		list := delegate.CreateObjectList()

		if target.Discovery.Selector.Labels != "" {
			selector, err := labels.Parse(target.Discovery.Selector.Labels)
			if err == nil {
				opts.LabelSelector = selector
			}
		}

		if target.Discovery.Selector.Fields != "" {
			selector, err := fields.ParseSelector(target.Discovery.Selector.Fields)
			if err == nil {
				opts.FieldSelector = selector
			}
		}

		if err := client.List(ctx, list, opts); err != nil {
			return []apv1beta2.PlanCommandTargetStatus{}
		}

		return delegate.ObjectListToPlanCommandTargetStatus(list, appc.SignalPending)
	}

	return []apv1beta2.PlanCommandTargetStatus{}
}

// ensureNodesExist ensures that all nodes in the provided slice of `PlanCommandTarget` have
// associated objects as identified by the provided exists function. If all nodes have been
// determined to exist, `true` is included in the return.
func ensureNodesExist(nodes []apv1beta2.PlanCommandTargetStatus, exists SignalObjectExistsFunc) ([]apv1beta2.PlanCommandTargetStatus, bool) {
	var allAccountedFor = true

	for idx, node := range nodes {
		if found, status := exists(node.Name); !found {
			if status != nil {
				nodes[idx].State = *status
			}

			allAccountedFor = false
		}
	}

	return nodes, allAccountedFor
}
