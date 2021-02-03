package v1beta1

import (
	"github.com/stretchr/testify/require"
	"testing"
)

func TestAddDualStackArguments(t *testing.T) {
	ds := DualStack{Enabled: true}
	t.Run("If no extrargs given, just add DualStack", func(t *testing.T) {
		args := map[string]string{}
		ds.EnableDualStackFeatureGate(args)
		require.Equal(t, "IPv6DualStack=true", args["feature-gates"])
	})
	t.Run("keep already existed extra-args", func(t *testing.T) {
		args := map[string]string{
			"some-argument": "value",
		}
		ds.EnableDualStackFeatureGate(args)
		require.Equal(t, "IPv6DualStack=true", args["feature-gates"])
		require.Equal(t, "value", args["some-argument"])
	})
	t.Run("keep already existed extra-args feature gates", func(t *testing.T) {
		args := map[string]string{
			"feature-gates": "Magic=true",
		}
		ds.EnableDualStackFeatureGate(args)
		require.Equal(t, "Magic=true,IPv6DualStack=true", args["feature-gates"])
	})
	t.Run("do nothing if dual-stack disabled", func(t *testing.T) {
		ds := DualStack{}
		args := map[string]string{}
		ds.EnableDualStackFeatureGate(args)
		require.Empty(t, args)
	})
}
