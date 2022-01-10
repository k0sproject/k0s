/*
Copyright 2022 k0s authors

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
package airgap

import (
	"github.com/k0sproject/k0s/pkg/apis/k0s.k0sproject.io/v1beta1"
	"github.com/k0sproject/k0s/pkg/constant"
)

var pauseImage = v1beta1.ImageSpec{
	Image:   constant.KubePauseContainerImage,
	Version: constant.KubePauseContainerImageVersion,
}

// GetImageURIs returns all image tags
func GetImageURIs(spec *v1beta1.ClusterImages) []string {
	images := []string{
		spec.Konnectivity.URI(),
		spec.CoreDNS.URI(),
		spec.KubeProxy.URI(),
		spec.MetricsServer.URI(),
		pauseImage.URI(),
		spec.KubeRouter.CNI.URI(),
		spec.KubeRouter.CNIInstaller.URI(),
	}
	images = append(images,
		spec.Calico.CNI.URI(),
		spec.Calico.KubeControllers.URI(),
		spec.Calico.Node.URI())
	return images
}
