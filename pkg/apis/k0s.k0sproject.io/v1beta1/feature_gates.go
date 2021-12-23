package v1beta1

import (
	"fmt"

	"github.com/k0sproject/k0s/internal/pkg/stringmap"
)

const (
	ServiceInternalTrafficPolicyFeatureGate = "ServiceInternalTrafficPolicy"
	DualStackFeatureGate                    = "IPv6DualStack"
)

// EnableFeatureGate enables given feature gate in the arguments
func EnableFeatureGate(args stringmap.StringMap, gateName string) stringmap.StringMap {
	gateString := fmt.Sprintf("%s=true", gateName)
	fg, found := args["feature-gates"]
	if !found {
		args["feature-gates"] = gateString
	} else {
		fg = fg + "," + gateString
		args["feature-gates"] = fg
	}
	return args
}
