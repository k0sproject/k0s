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
