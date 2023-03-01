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
	"fmt"

	apv1beta2 "github.com/k0sproject/k0s/pkg/apis/autopilot/v1beta2"

	v1 "k8s.io/api/core/v1"
	crcli "sigs.k8s.io/controller-runtime/pkg/client"
	crman "sigs.k8s.io/controller-runtime/pkg/manager"
)

// RegisterIndexers registers all required common indexers into the controller-runtime manager.
func RegisterIndexers(ctx context.Context, mgr crman.Manager, scope string) error {
	var indicies = []struct {
		field   string
		object  crcli.Object
		scope   string
		indexer crcli.IndexerFunc
	}{
		{
			"spec.id",
			&apv1beta2.Plan{},
			"",
			func(obj crcli.Object) []string {
				if plan, ok := obj.(*apv1beta2.Plan); ok {
					return []string{plan.Spec.ID}
				}

				return nil
			},
		},
		{
			"metadata.name",
			&apv1beta2.ControlNode{},
			"controller",
			func(obj crcli.Object) []string {
				if cn, ok := obj.(*apv1beta2.ControlNode); ok {
					return []string{cn.Name}
				}

				return nil
			},
		},
		{
			"metadata.name",
			&v1.Node{},
			"worker",
			func(obj crcli.Object) []string {
				if n, ok := obj.(*v1.Node); ok {
					return []string{n.Name}
				}

				return nil
			},
		},
	}

	for _, index := range indicies {
		// Worker autopilot instances shouldn't need to setup indicies for controller
		// types that they'll never use.
		if scope == "worker" && index.scope == "controller" {
			continue
		}

		if err := mgr.GetFieldIndexer().IndexField(ctx, index.object, index.field, index.indexer); err != nil {
			return fmt.Errorf("unable to register indexer '%s' on '%v': %w", index.field, index.object.GetObjectKind(), err)
		}
	}

	return nil
}
