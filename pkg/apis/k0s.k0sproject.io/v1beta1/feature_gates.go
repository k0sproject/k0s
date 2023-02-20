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
	"fmt"

	"github.com/k0sproject/k0s/internal/pkg/stringmap"
)

const (
	ServiceInternalTrafficPolicyFeatureGate = "ServiceInternalTrafficPolicy"
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
