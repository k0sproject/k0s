// SPDX-FileCopyrightText: 2021 k0s authors
// SPDX-License-Identifier: Apache-2.0

package constant

const (
	AutopilotName                      = "autopilot"
	AutopilotNamespace                 = "k0s-autopilot"
	K0sTempFilename                    = "k0s.tmp"
	CentralCordoningLabel              = "autopilot.k0sproject.io/central-cordoning"
	K0SControlNodeModeAnnotation       = "autopilot.k0sproject.io/mode"
	K0SControlNodeModeController       = "controller"
	K0SControlNodeModeControllerWorker = "controller+worker"
)
