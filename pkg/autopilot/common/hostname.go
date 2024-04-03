// Copyright 2021 k0s authors
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

package common

import (
	"os"

	nodeutil "k8s.io/kubernetes/pkg/util/node"
)

const (
	envAutopilotHostname = "AUTOPILOT_HOSTNAME"
)

// FindEffectiveHostname attempts to find the effective hostname, first inspecting
// for an AUTOPILOT_HOSTNAME environment variable, falling back to whatever the OS
// returns.
func FindEffectiveHostname() (string, error) {
	// nodeutil.GetHostname will return the "object name safe" hostname, overridden via the env var if non-empty value
	return nodeutil.GetHostname(os.Getenv(envAutopilotHostname))
}
