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
	"fmt"

	apcomm "github.com/k0sproject/k0s/pkg/autopilot/common"

	v1 "k8s.io/api/core/v1"
	crcli "sigs.k8s.io/controller-runtime/pkg/client"
)

// signalNodePlatformIdentifier inspects the signal nodes labels and returns a
// platform identifier.
func SignalNodePlatformIdentifier(obj crcli.Object) (string, error) {
	if labels := obj.GetLabels(); labels != nil {
		arch, archOk := labels[v1.LabelArchStable]
		os, osOk := labels[v1.LabelOSStable]

		if archOk && osOk {
			return apcomm.PlatformIdentifier(os, arch), nil
		}
	}

	return "", fmt.Errorf("unable to determine platform identifier for '%s'", obj.GetName())
}
