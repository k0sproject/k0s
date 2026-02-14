// SPDX-FileCopyrightText: 2022 k0s authors
// SPDX-License-Identifier: Apache-2.0

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
