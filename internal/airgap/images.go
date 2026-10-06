// SPDX-FileCopyrightText: 2021 k0s authors
// SPDX-License-Identifier: Apache-2.0

package airgap

import (
	"cmp"

	"github.com/k0sproject/k0s/pkg/apis/k0s/v1beta1"

	imagespecv1 "github.com/opencontainers/image-spec/specs-go/v1"
)

// Describes the environment to generate an image list for, i.e. the k0s node's
// platform and configuration.
type TargetEnv struct {
	// The platform of the node to generate the image list for.
	Platform imagespecv1.Platform

	// The cluster configuration to select images from. Note that this must be
	// properly defaulted.
	Spec *v1beta1.ClusterSpec
}

// GetImageURIs returns the image URIs that match the given env. If all is
// specified, all images will be included in the returned list, no matter if
// they're used in the current environment's configuration or not.
func GetImageURIs(env TargetEnv, all bool) (uris []string) {
	// Add base images
	switch env.Platform.OS {
	case "linux":
		// Enabled by default.
		uris = append(uris,
			env.Spec.Images.Pause.URI(),
			env.Spec.Images.Konnectivity.URI(),
			env.Spec.Images.KubeProxy.URI(),
			env.Spec.Images.CoreDNS.URI(),
			env.Spec.Images.MetricsServer.URI(),
		)

		// Include disabled-by default images, if the user wants all images.
		if all {
			uris = append(uris, env.Spec.Images.PushGateway.URI())
		}

	case "windows":
		// Enabled by default.
		uris = append(uris,
			env.Spec.Images.Windows.Pause.URI(),
			env.Spec.Images.Windows.KubeProxy.URI(),
		)
	}

	if all || env.wantsNetworkProvider("kuberouter") {
		switch env.Platform.OS {
		case "linux":
			uris = append(uris,
				env.Spec.Images.KubeRouter.CNIInstaller.URI(),
				env.Spec.Images.KubeRouter.CNI.URI(),
			)
		}
	}

	if all || env.wantsNetworkProvider("calico") {
		switch env.Platform.OS {
		case "linux":
			uris = append(uris,
				env.Spec.Images.Calico.CNI.URI(),
				env.Spec.Images.Calico.KubeControllers.URI(),
				env.Spec.Images.Calico.Node.URI(),
			)
		case "windows":
			uris = append(uris,
				env.Spec.Images.Calico.Windows.CNI.URI(),
				env.Spec.Images.Calico.Windows.Node.URI(),
			)
		}
	}

	if all || env.wantsNLLBBackend(v1beta1.NllbTypeEnvoyProxy) {
		switch env.Platform.OS {
		case "linux":
			switch env.Platform.Architecture {
			case "arm", "riscv64":
			default:
				uris = append(uris,
					env.Spec.Network.NodeLocalLoadBalancing.EnvoyProxy.OrDefault().Image.URI(),
				)
			}
		}
	}

	if all || env.wantsNLLBBackend(v1beta1.NllbTypeTraefik) {
		uris = append(uris,
			env.Spec.Network.NodeLocalLoadBalancing.Traefik.OrDefault().Image.URI(),
		)
	}

	return
}

func (e *TargetEnv) wantsNetworkProvider(provider string) bool {
	usedProvider := "kuberouter"
	if e.Spec != nil && e.Spec.Network != nil {
		usedProvider = cmp.Or(e.Spec.Network.Provider, usedProvider)
	}

	return provider == usedProvider
}

func (e *TargetEnv) wantsNLLBBackend(backend v1beta1.NllbType) bool {
	var nllbType v1beta1.NllbType
	if e.Spec != nil && e.Spec.Network != nil {
		if nllb := e.Spec.Network.NodeLocalLoadBalancing; nllb.IsEnabled() {
			nllbType = cmp.Or(nllb.Type, v1beta1.NllbTypeEnvoyProxy)
		}
	}

	return nllbType == backend
}
