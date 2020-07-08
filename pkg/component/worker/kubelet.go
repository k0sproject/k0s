package worker

import (
	"fmt"
	"path"

	"github.com/Mirantis/mke/pkg/assets"
	"github.com/Mirantis/mke/pkg/constant"
	"github.com/Mirantis/mke/pkg/supervisor"
	"github.com/Mirantis/mke/pkg/util"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

const kubeletConfig = `
apiVersion: kubelet.config.k8s.io/v1beta1
kind: KubeletConfiguration
staticPodPath: /etc/kubernetes/manifests
authentication:
  anonymous:
    enabled: false
  webhook:
    enabled: true
    cacheTTL: "2m"
  x509:
    clientCAFile: /var/lib/mke/pki/ca.crt
authorization:
  mode: Webhook
  webhook:
    cacheAuthorizedTTL: "5m"
    cacheUnauthorizedTTL: "30s"
`

const kubeletConfigPath = "/var/lib/mke/kubelet-config.yaml"

type Kubelet struct {
	supervisor supervisor.Supervisor
}

type KubeletConfig struct {
}

// Init extracts the needed binaries
func (k *Kubelet) Init() error {
	return assets.Stage(constant.DataDir, path.Join("bin", "kubelet"))
}

// Run runs kubelet
func (k *Kubelet) Run() error {
	logrus.Info("Starting containerD")
	k.supervisor = supervisor.Supervisor{
		Name:    "kubelet",
		BinPath: path.Join(constant.DataDir, "bin", "kubelet"),
		Args: []string{
			"--container-runtime=remote",
			"--container-runtime-endpoint=unix:///run/containerd/containerd.sock",
			fmt.Sprintf("--config=%s", kubeletConfigPath),
			fmt.Sprintf("--bootstrap-kubeconfig=%s", constant.KubeletBootstrapConfigPath),
			"--kubeconfig=/var/lib/mke/kubelet.conf",
		},
	}
	// TODO Make proper kubelet config
	tw := util.TemplateWriter{
		Name:     "kubeletConfig",
		Template: kubeletConfig,
		Data:     KubeletConfig{},
		Path:     kubeletConfigPath,
	}

	err := tw.Write()
	if err != nil {
		return errors.Wrapf(err, "failed to create kubelet config file")
	}

	k.supervisor.Supervise()

	return nil
}

// Stop stops kubelet
func (k *Kubelet) Stop() error {
	return k.supervisor.Stop()
}
