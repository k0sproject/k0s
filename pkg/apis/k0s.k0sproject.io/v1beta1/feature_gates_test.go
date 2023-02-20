/*
Copyright 2021 k0s authors

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v1beta1

import (
	"testing"

	"github.com/k0sproject/k0s/internal/pkg/stringmap"
	"github.com/stretchr/testify/require"
)

func TestArgsFeatureGates(t *testing.T) {
	t.Run("if_no_feature_gates_add_new_one", func(t *testing.T) {
		args := stringmap.StringMap{}
		EnableFeatureGate(args, ServiceInternalTrafficPolicyFeatureGate)
		require.Equal(t, "ServiceInternalTrafficPolicy=true", args["feature-gates"])
	})
	t.Run("if_args_has_some_argument_preserve_it", func(t *testing.T) {
		args := stringmap.StringMap{
			"some-argument": "value",
		}
		EnableFeatureGate(args, ServiceInternalTrafficPolicyFeatureGate)
		require.Equal(t, "ServiceInternalTrafficPolicy=true", args["feature-gates"])
		require.Equal(t, "value", args["some-argument"])
	})
	t.Run("merge_new_feature_gate_with_the_current", func(t *testing.T) {
		args := stringmap.StringMap{
			"feature-gates": "Magic=true",
		}
		EnableFeatureGate(args, ServiceInternalTrafficPolicyFeatureGate)
		require.Equal(t, "Magic=true,ServiceInternalTrafficPolicy=true", args["feature-gates"])
	})
}
