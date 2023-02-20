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

package controller

import (
	"path"

	"github.com/k0sproject/k0s/internal/pkg/file"
)

func existingCNIProvider(manifestDir string) string {
	calicoManifestPath := path.Join(manifestDir, "calico", "calico-DaemonSet-calico-node.yaml")
	if file.Exists(calicoManifestPath) {
		return "calico"
	}

	kubeRouterManifestPath := path.Join(manifestDir, "kuberouter", "kube-router.yaml")
	if file.Exists(kubeRouterManifestPath) {
		return "kuberouter"
	}

	return ""
}
