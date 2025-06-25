// SPDX-FileCopyrightText: 2021 k0s authors
// SPDX-License-Identifier: Apache-2.0

package common

import (
	"os"

	"github.com/k0sproject/k0s/internal/pkg/flags"
	"github.com/k0sproject/k0s/pkg/node"
)

const (
	envAutopilotHostname = "AUTOPILOT_HOSTNAME"
)

// FindEffectiveHostname attempts to find the effective hostname, first inspecting
// for an AUTOPILOT_HOSTNAME environment variable, falling back to whatever the OS
// returns.
func FindEffectiveHostname() (string, error) {
	nodeName, err := node.GetNodeName(os.Getenv(envAutopilotHostname))
	return string(nodeName), err
}

func FindKubeletHostname(kubeletExtraArgs string) string {
	defaultNodename, _ := node.GetNodeName("")
	if kubeletExtraArgs != "" {
		extras := flags.Split(kubeletExtraArgs)
		nodeName, ok := extras["--hostname-override"]
		if ok {
			return nodeName
		}
	}

	return string(defaultNodename)
}
