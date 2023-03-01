// Copyright 2022 k0s authors
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

package utils

import (
	"context"

	apv1beta2 "github.com/k0sproject/k0s/pkg/apis/autopilot/v1beta2"
	appc "github.com/k0sproject/k0s/pkg/autopilot/controller/plans/core"

	"k8s.io/apimachinery/pkg/types"
	crcli "sigs.k8s.io/controller-runtime/pkg/client"
)

// objectExistsWithPlatform looks up an object for a given name and type, and determines
// if there is a platform available for it in the provided plan.
func ObjectExistsWithPlatform(ctx context.Context, client crcli.Client, name string, obj crcli.Object, platformMap apv1beta2.PlanPlatformResourceURLMap) (bool, *apv1beta2.PlanCommandTargetStateType) {
	key := types.NamespacedName{Name: name}
	if err := client.Get(ctx, key, obj); err != nil {
		return false, &appc.SignalMissingNode
	}

	// Determine what platform this signal node is
	platformID, err := SignalNodePlatformIdentifier(obj)
	if err != nil {
		return false, &appc.SignalMissingPlatform
	}

	// Ensure that the plan has a platform matching this signal node
	if _, found := platformMap[platformID]; !found {
		return false, &appc.SignalMissingPlatform
	}

	return true, nil
}
