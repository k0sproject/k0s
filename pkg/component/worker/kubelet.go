package worker

import (
	"fmt"
	"os"
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
failSwapOn: false
`

const (
	kubeletConfigPath      = "/var/lib/mke/kubelet-config.yaml"
	kubeletVolumePluginDir = "/usr/libexec/mke/kubelet-plugins/volume/exec"
)

type Kubelet struct {
	supervisor      supervisor.Supervisor
	dataDir         string
	volumePluginDir string
}

type KubeletConfig struct {
}

// Init extracts the needed binaries
func (k *Kubelet) Init() error {
	err := assets.Stage(constant.DataDir, path.Join("bin", "kubelet"), constant.Group)
	if err != nil {
		return err
	}

	k.dataDir = path.Join(constant.DataDir, "kubelet")
	err = os.MkdirAll(k.dataDir, 0700)
	if err != nil {
		return errors.Wrapf(err, "failed to create %s", k.dataDir)
	}

	err = os.MkdirAll(kubeletVolumePluginDir, 0700)
	if err != nil {
		return errors.Wrapf(err, "failed to create %s", kubeletVolumePluginDir)
	}

	return nil
}

// Run runs kubelet
func (k *Kubelet) Run() error {
	logrus.Info("Starting containerD")
	k.supervisor = supervisor.Supervisor{
		Name:    "kubelet",
		BinPath: assets.StagedBinPath(constant.DataDir, "kubelet"),
		Args: []string{
			fmt.Sprintf("--root-dir=%s", k.dataDir),
			fmt.Sprintf("--volume-plugin-dir=%s", kubeletVolumePluginDir),
			"--container-runtime=remote",
			"--container-runtime-endpoint=unix:///run/mke/containerd.sock",
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
