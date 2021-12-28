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
