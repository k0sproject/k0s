package v1beta1

import (
	"testing"

	"github.com/k0sproject/k0s/internal/pkg/stringmap"
	"github.com/stretchr/testify/require"
)

func TestArgsFeatureGates(t *testing.T) {
	t.Run("if_no_feature_gates_add_new_one", func(t *testing.T) {
		args := stringmap.StringMap{}
		EnableFeatureGate(args, DualStackFeatureGate)
		require.Equal(t, "IPv6DualStack=true", args["feature-gates"])
	})
	t.Run("if_args_has_some_argument_preserve_it", func(t *testing.T) {
		args := stringmap.StringMap{
			"some-argument": "value",
		}
		EnableFeatureGate(args, DualStackFeatureGate)
		require.Equal(t, "IPv6DualStack=true", args["feature-gates"])
		require.Equal(t, "value", args["some-argument"])
	})
	t.Run("merge_new_feature_gate_with_the_current", func(t *testing.T) {
		args := stringmap.StringMap{
			"feature-gates": "Magic=true",
		}
		EnableFeatureGate(args, DualStackFeatureGate)
		require.Equal(t, "Magic=true,IPv6DualStack=true", args["feature-gates"])
	})
}
