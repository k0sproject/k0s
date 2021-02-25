package airgap

import (
	"github.com/k0sproject/k0s/pkg/apis/v1beta1"
	"github.com/k0sproject/k0s/pkg/constant"
)

var images = []v1beta1.ImageSpec{
	{
		Image:   constant.KonnectivityImage,
		Version: constant.KonnectivityImageVersion,
	},
	{
		Image:   constant.MetricsImage,
		Version: constant.MetricsImageVersion,
	},
	{
		Image:   constant.KubeProxyImage,
		Version: constant.KubeProxyImageVersion,
	},
	{
		Image:   constant.CoreDNSImage,
		Version: constant.CoreDNSImageVersion,
	},
	{
		Image:   constant.CalicoImage,
		Version: constant.CalicoImageVersion,
	},
	{
		Image:   constant.CalicoNodeImage,
		Version: constant.CalicoNodeImageVersion,
	},
	{
		Image:   constant.KubeControllerImage,
		Version: constant.KubeControllerImageVersion,
	},
	{
		Image:   constant.KubePauseContainerImage,
		Version: constant.KubePauseContainerImageVersion,
	},
}

// GetImageURIs returns all image tags
func GetImageURIs() []string {
	imageNameAndVersions := make([]string, 0, len(images))
	for _, i := range images {
		imageNameAndVersions = append(imageNameAndVersions, i.URI())
	}
	return imageNameAndVersions
}
