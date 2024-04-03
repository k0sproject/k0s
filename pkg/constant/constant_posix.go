//go:build !windows
// +build !windows

/*
Copyright 2020 k0s authors

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

package constant

import "fmt"

const (
	// DataDirDefault is the default data directory containing k0s state
	DataDirDefault                 = "/var/lib/k0s"
	KubeletVolumePluginDir         = "/usr/libexec/k0s/kubelet-plugins/volume/exec"
	KineSocket                     = "kine/kine.sock:2379"
	KubePauseContainerImage        = "registry.k8s.io/pause"
	KubePauseContainerImageVersion = "3.6"
	K0sConfigPathDefault           = "/etc/k0s/k0s.yaml"
)

func formatPath(dir string, file string) string {
	return fmt.Sprintf("%s/%s", dir, file)
}
