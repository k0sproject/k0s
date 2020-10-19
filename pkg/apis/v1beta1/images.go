package v1beta1

import "fmt"

const (
	ImagePullPolicy = "pull"
)

type ImageSpec struct {
	Policy  string `yaml:"policy"`
	Image   string `yaml:"image"`
	Version string `yaml:"version"`
}

// URI build image uri
func (is ImageSpec) URI() string {
	return fmt.Sprintf("%s:%s", is.Image, is.Version)
}

// ClusterImages sets docker images for addon components
type ClusterImages struct {
	Konnectivity  ImageSpec
	MetricsServer ImageSpec
	KubeProxy     ImageSpec
	CoreDNS       ImageSpec
}

// DefaultClusterImages default image settings
func DefaultClusterImages() *ClusterImages {
	// TODO: add calico, it's harder because we don't control manifests directly
	return &ClusterImages{
		Konnectivity: ImageSpec{
			Policy:  ImagePullPolicy,
			Image:   "us.gcr.io/k8s-artifacts-prod/kas-network-proxy/proxy-agent",
			Version: "v0.0.12",
		},
		MetricsServer: ImageSpec{
			Policy:  ImagePullPolicy,
			Image:   "gcr.io/k8s-staging-metrics-server/metrics-server",
			Version: "v0.3.7",
		},
		KubeProxy: ImageSpec{
			Policy:  ImagePullPolicy,
			Image:   "k8s.gcr.io/kube-proxy",
			Version: "v1.19.0",
		},
		CoreDNS: ImageSpec{
			Policy:  ImagePullPolicy,
			Image:   "docker.io/coredns/coredns",
			Version: "1.7.0",
		},
	}
}
