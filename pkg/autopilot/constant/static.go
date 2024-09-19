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

package constant

const (
	AutopilotName                      = "autopilot"
	AutopilotNamespace                 = "k0s-autopilot"
	AutopilotConfigName                = AutopilotName
	K0sBinaryDir                       = "/usr/local/bin"
	K0sTempFilename                    = "k0s.tmp"
	K0sDefaultDataDir                  = "/var/lib/k0s"
	K0sManifestSubDir                  = "manifests"
	K0sImagesDir                       = "images"
	K0SControlNodeModeAnnotation       = "autopilot.k0sproject.io/mode"
	K0SControlNodeModeController       = "controller"
	K0SControlNodeModeControllerWorker = "controller+worker"
)
