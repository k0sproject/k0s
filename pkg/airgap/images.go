// SPDX-FileCopyrightText: 2021 k0s authors
// SPDX-License-Identifier: Apache-2.0

package airgap

import (
	"runtime"

	"github.com/k0sproject/k0s/pkg/apis/k0s/v1beta1"
)

// GetImageURIs returns all image tags
func GetImageURIs(spec *v1beta1.ClusterSpec, all bool) []string {

	imageURIs := []string{
		spec.Images.Calico.CNI.URI(),
		spec.Images.Calico.KubeControllers.URI(),
		spec.Images.Calico.Node.URI(),
		spec.Images.CoreDNS.URI(),
		spec.Images.Konnectivity.URI(),
		spec.Images.KubeProxy.URI(),
		spec.Images.KubeRouter.CNI.URI(),
		spec.Images.KubeRouter.CNIInstaller.URI(),
		spec.Images.MetricsServer.URI(),
		spec.Images.Pause.URI(),
	}

	if all {
		// Currently we can't determine if the user has enabled the PushGateway via
		// config so include it only if all is requested
		imageURIs = append(imageURIs,
			spec.Images.PushGateway.URI(),
		)
	}

	if spec.Network != nil {
		nllb := spec.Network.NodeLocalLoadBalancing
		if nllb != nil && (all || nllb.IsEnabled()) {
			switch nllb.Type {
			case v1beta1.NllbTypeEnvoyProxy:
				if runtime.GOARCH != "arm" && nllb.EnvoyProxy != nil && nllb.EnvoyProxy.Image != nil {
					imageURIs = append(imageURIs, nllb.EnvoyProxy.Image.URI())
				}
			}
		}
	}

	return imageURIs
}
