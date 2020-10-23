package v1beta1

import (
	"fmt"
	"strings"

	"github.com/Mirantis/mke/pkg/constant"
)

// ImageSpec container image settings
type ImageSpec struct {
	Image   string `yaml:"image"`
	Version string `yaml:"version"`
}

// URI build image uri
func (is ImageSpec) URI() string {
	return fmt.Sprintf("%s:%s", is.Image, is.Version)
}

// ClusterImages sets docker images for addon components
type ClusterImages struct {
	Konnectivity  ImageSpec `yaml:"konnectivity"`
	MetricsServer ImageSpec `yaml:"metricsserver"`
	KubeProxy     ImageSpec `yaml:"kubeproxy"`
	CoreDNS       ImageSpec `yaml:"coredns"`

	Calico CalicoImageSpec `yaml:"calico"`

	Repository string `yaml:"repository"`
}

func (ci *ClusterImages) UnmarshalYAML(unmarshal func(interface{}) error) error {
	type wrapper ClusterImages
	imagesWrapper := (*wrapper)(ci)
	if err := unmarshal(imagesWrapper); err != nil {
		return err
	}
	ci.overrideImageRepositories()

	return nil
}

func (ci *ClusterImages) overrideImageRepositories() {
	if ci.Repository == "" {
		return
	}
	override := func(dst *ImageSpec) {
		dst.Image = overrideRepository(ci.Repository, dst.Image)
	}
	override(&ci.Konnectivity)
	override(&ci.MetricsServer)
	override(&ci.KubeProxy)
	override(&ci.CoreDNS)
	override(&ci.Calico.CNI)
	override(&ci.Calico.FlexVolume)
	override(&ci.Calico.Node)
	override(&ci.Calico.KubeControllers)
}

// CalicoImageSpec config group for calico related image settings
type CalicoImageSpec struct {
	CNI             ImageSpec `yaml:"cni"`
	FlexVolume      ImageSpec `yaml:"flexvolume"`
	Node            ImageSpec `yaml:"node"`
	KubeControllers ImageSpec `yaml:"kubecontrollers"`
}

// DefaultClusterImages default image settings
func DefaultClusterImages() *ClusterImages {
	return &ClusterImages{
		Konnectivity: ImageSpec{
			Image:   constant.KonnectivityImage,
			Version: constant.KonnectivityImageVersion,
		},
		MetricsServer: ImageSpec{
			Image:   constant.MetricsImage,
			Version: constant.MetricsImageVersion,
		},
		KubeProxy: ImageSpec{
			Image:   constant.KubeProxyImage,
			Version: constant.KubeProxyImageVersion,
		},
		CoreDNS: ImageSpec{
			Image:   constant.CoreDNSImage,
			Version: constant.CoreDNSImageVersion,
		},
		Calico: CalicoImageSpec{
			CNI: ImageSpec{
				Image:   constant.CalicoImage,
				Version: constant.CalicoImageVersion,
			},
			FlexVolume: ImageSpec{
				Image:   constant.FlexVolumeImage,
				Version: constant.FlexVolumeImageVersion,
			},
			Node: ImageSpec{
				Image:   constant.CalicoNodeImage,
				Version: constant.CalicoNodeImageVersion,
			},
			KubeControllers: ImageSpec{
				Image:   constant.KubeControllerImage,
				Version: constant.KubeControllerImageVersion,
			},
		},
	}
}

func getHostName(imageName string) string {
	i := strings.IndexRune(imageName, '/')
	if i == -1 || (!strings.ContainsAny(imageName[:i], ".:") && imageName[:i] != "localhost") {
		// we have no domain in this ref
		return ""
	}
	return imageName[:i]
}

func overrideRepository(repository string, originalImage string) string {
	if host := getHostName(originalImage); host != "" {
		return strings.Replace(originalImage, host, repository, 1)
	}
	return fmt.Sprintf("%s/%s", repository, originalImage)
}
