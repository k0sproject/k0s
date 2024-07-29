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

package v1beta1

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/k0sproject/k0s/pkg/constant"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/validation/field"

	"github.com/distribution/reference"
)

// ImageSpec container image settings
type ImageSpec struct {
	// +kubebuilder:validation:MinLength=1
	Image string `json:"image"`

	// +kubebuilder:validation:Pattern="^[\\w][\\w.-]{0,127}(?:@[A-Za-z][A-Za-z0-9]*(?:[-_+.][A-Za-z][A-Za-z0-9]*)*[:][[:xdigit:]]{32,})?$"
	Version string `json:"version"`
}

func (s *ImageSpec) Validate(path *field.Path) (errs field.ErrorList) {
	if s == nil {
		return
	}

	imageLen := len(s.Image)
	if imageLen == 0 {
		errs = append(errs, field.Required(path.Child("image"), ""))
	} else if imageLen != len(strings.TrimSpace(s.Image)) {
		errs = append(errs, field.Invalid(path.Child("image"), s.Image, "must not have leading or trailing whitespace"))
	}

	// Validate the image contains a tag and optional digest
	versionRe := regexp.MustCompile(`^` + reference.TagRegexp.String() + `(?:@` + reference.DigestRegexp.String() + `)?$`)
	if !versionRe.MatchString(s.Version) {
		errs = append(errs, field.Invalid(path.Child("version"), s.Version, "must match regular expression: "+versionRe.String()))
	}

	return
}

// URI build image uri
func (s *ImageSpec) URI() string {
	return fmt.Sprintf("%s:%s", s.Image, s.Version)
}

// ClusterImages sets docker images for addon components
type ClusterImages struct {
	Konnectivity  ImageSpec `json:"konnectivity,omitempty"`
	PushGateway   ImageSpec `json:"pushgateway,omitempty"`
	MetricsServer ImageSpec `json:"metricsserver,omitempty"`
	KubeProxy     ImageSpec `json:"kubeproxy,omitempty"`
	CoreDNS       ImageSpec `json:"coredns,omitempty"`
	Pause         ImageSpec `json:"pause,omitempty"`

	Calico     CalicoImageSpec     `json:"calico,omitempty"`
	KubeRouter KubeRouterImageSpec `json:"kuberouter,omitempty"`

	Repository string `json:"repository,omitempty"`

	// +kubebuilder:default=IfNotPresent
	// +kubebuilder:validation:Enum=Always;IfNotPresent;Never
	DefaultPullPolicy string `json:"default_pull_policy,omitempty"`
}

func (ci *ClusterImages) UnmarshalJSON(data []byte) error {
	type images ClusterImages
	imagesWrapper := (*images)(ci)
	if err := json.Unmarshal(data, imagesWrapper); err != nil {
		return err
	}
	ci.overrideImageRepositories()
	if ci.DefaultPullPolicy == "" {
		ci.DefaultPullPolicy = string(corev1.PullIfNotPresent)
	}
	return nil
}

func (ci *ClusterImages) Validate(path *field.Path) (errs field.ErrorList) {
	if ci == nil {
		return
	}

	defaultPullPolicy := corev1.PullPolicy(ci.DefaultPullPolicy)
	switch defaultPullPolicy {
	case corev1.PullAlways, corev1.PullIfNotPresent, corev1.PullNever:
		break
	case "":
		errs = append(errs, field.Required(path.Child("default_pull_policy"), ""))
	default:
		errs = append(errs, field.NotSupported(field.NewPath("default_pull_policy"), defaultPullPolicy, []string{
			string(corev1.PullAlways),
			string(corev1.PullIfNotPresent),
			string(corev1.PullNever),
		}))
	}

	errs = append(errs, ci.Konnectivity.Validate(path.Child("konnectivity"))...)
	errs = append(errs, ci.PushGateway.Validate(path.Child("pushgateway"))...)
	errs = append(errs, ci.MetricsServer.Validate(path.Child("metricsserver"))...)
	errs = append(errs, ci.KubeProxy.Validate(path.Child("kubeproxy"))...)
	errs = append(errs, ci.CoreDNS.Validate(path.Child("coredns"))...)
	errs = append(errs, ci.Pause.Validate(path.Child("pause"))...)
	errs = append(errs, ci.Calico.Validate(path.Child("calico"))...)
	errs = append(errs, ci.KubeRouter.Validate(path.Child("kuberouter"))...)
	return
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
	override(&ci.Pause)
}

// CalicoImageSpec config group for calico related image settings
type CalicoImageSpec struct {
	CNI             ImageSpec `json:"cni,omitempty"`
	Node            ImageSpec `json:"node,omitempty"`
	KubeControllers ImageSpec `json:"kubecontrollers,omitempty"`
}

func (s *CalicoImageSpec) Validate(path *field.Path) (errs field.ErrorList) {
	if s == nil {
		return
	}
	errs = append(errs, s.CNI.Validate(path.Child("cni"))...)
	errs = append(errs, s.Node.Validate(path.Child("node"))...)
	errs = append(errs, s.KubeControllers.Validate(path.Child("kubecontrollers"))...)
	return
}

// KubeRouterImageSpec config group for kube-router related images
type KubeRouterImageSpec struct {
	CNI          ImageSpec `json:"cni,omitempty"`
	CNIInstaller ImageSpec `json:"cniInstaller,omitempty"`
}

func (s *KubeRouterImageSpec) Validate(path *field.Path) (errs field.ErrorList) {
	if s == nil {
		return
	}
	errs = append(errs, s.CNI.Validate(path.Child("cni"))...)
	errs = append(errs, s.CNIInstaller.Validate(path.Child("cniInstaller"))...)
	return
}

// DefaultClusterImages default image settings
func DefaultClusterImages() *ClusterImages {
	return &ClusterImages{
		DefaultPullPolicy: "IfNotPresent",
		Konnectivity: ImageSpec{
			Image:   constant.KonnectivityImage,
			Version: constant.KonnectivityImageVersion,
		},
		PushGateway: ImageSpec{
			Image:   constant.PushGatewayImage,
			Version: constant.PushGatewayImageVersion,
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
		Pause: ImageSpec{
			Image:   constant.KubePauseContainerImage,
			Version: constant.KubePauseContainerImageVersion,
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
