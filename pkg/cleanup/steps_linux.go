//go:build linux

// SPDX-FileCopyrightText: 2025 k0s authors
// SPDX-License-Identifier: Apache-2.0

package cleanup

import (
	k0sv1beta1 "github.com/k0sproject/k0s/pkg/apis/k0s/v1beta1"
	"github.com/k0sproject/k0s/pkg/config"
)

func buildSteps(debug bool, k0sVars *config.CfgVars, systemUsers *k0sv1beta1.SystemUser, criSocketFlag string) ([]Step, error) {
	containers, err := newContainersStep(debug, k0sVars, criSocketFlag)
	if err != nil {
		return nil, err
	}

	steps := []Step{
		containers,
		&users{systemUsers: systemUsers},
		&services{},
		&directories{
			dataDir:        k0sVars.DataDir,
			kubeletRootDir: k0sVars.KubeletRootDir,
			runDir:         k0sVars.RunDir,
		},
		&cni{},
	}

	if bridge := newBridgeStep(); bridge != nil {
		steps = append(steps, bridge)
	}

	return steps, nil
}
