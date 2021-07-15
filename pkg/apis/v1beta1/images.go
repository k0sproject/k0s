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
package v1beta1

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/k0sproject/k0s/pkg/constant"
)

// ImageSpec container image settings
type ImageSpec struct {
	Image   string `json:"image"`
	Version string `json:"version"`
}

// URI build image uri
func (is ImageSpec) URI() string {
	return fmt.Sprintf("%s:%s", is.Image, is.Version)
}

// ClusterImages sets docker images for addon components
type ClusterImages struct {
	Konnectivity  ImageSpec `json:"konnectivity"`
	MetricsServer ImageSpec `json:"metricsserver"`
	KubeProxy     ImageSpec `json:"kubeproxy"`
	CoreDNS       ImageSpec `json:"coredns"`

	Calico     CalicoImageSpec     `json:"calico"`
	KubeRouter KubeRouterImageSpec `json:"kuberouter"`

	Repository        string `json:"repository,omitempty"`
	DefaultPullPolicy string `json:"default_pull_policy,omitempty"`
}

func (ci *ClusterImages) UnmarshalJSON(data []byte) error {
	type wrapper ClusterImages
	imagesWrapper := (*wrapper)(ci)
	if err := json.Unmarshal(data, imagesWrapper); err != nil {
		return err
	}
	ci.overrideImageRepositories()
	ci.DefaultPullPolicy = "IfNotPresent"
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
	override(&ci.Calico.Node)
	override(&ci.Calico.KubeControllers)
	override(&ci.KubeRouter.CNI)
	override(&ci.KubeRouter.CNIInstaller)
}

// CalicoImageSpec config group for calico related image settings
type CalicoImageSpec struct {
	CNI             ImageSpec `json:"cni"`
	Node            ImageSpec `json:"node"`
	KubeControllers ImageSpec `json:"kubecontrollers"`
}

// KubeRouterImageSpec config group for kube-router related images
type KubeRouterImageSpec struct {
	CNI          ImageSpec `json:"cni"`
	CNIInstaller ImageSpec `json:"cniInstaller"`
}

// DefaultClusterImages default image settings
func DefaultClusterImages() *ClusterImages {
	return &ClusterImages{
		DefaultPullPolicy: "IfNotPresent",
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
				Version: constant.CalicoComponentImagesVersion,
			},
			Node: ImageSpec{
				Image:   constant.CalicoNodeImage,
				Version: constant.CalicoComponentImagesVersion,
			},
			KubeControllers: ImageSpec{
				Image:   constant.KubeControllerImage,
				Version: constant.CalicoComponentImagesVersion,
			},
		},
		KubeRouter: KubeRouterImageSpec{
			CNI: ImageSpec{
				Image:   constant.KubeRouterCNIImage,
				Version: constant.KubeRouterCNIImageVersion,
			},
			CNIInstaller: ImageSpec{
				Image:   constant.KubeRouterCNIInstallerImage,
				Version: constant.KubeRouterCNIInstallerImageVersion,
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

// Validate stub for Validateable interface
func (ci *ClusterImages) Validate() []error {
	return nil
}
