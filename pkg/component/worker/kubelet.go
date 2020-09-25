package worker

import (
	"fmt"
	"io/ioutil"
	"os"
	"path"

	"github.com/Mirantis/mke/pkg/assets"
	"github.com/Mirantis/mke/pkg/constant"
	"github.com/Mirantis/mke/pkg/supervisor"
	"github.com/avast/retry-go"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

const (
	kubeletConfigPath      = "/var/lib/mke/kubelet-config.yaml"
	kubeletVolumePluginDir = "/usr/libexec/mke/kubelet-plugins/volume/exec"
)

// Kubelet is the component implementation to manage kubelet
type Kubelet struct {
	KubeletConfigClient *KubeletConfigClient

	supervisor      supervisor.Supervisor
	dataDir         string
	volumePluginDir string
}

// KubeletConfig defines the kubelet related config options
type KubeletConfig struct {
	ClusterDNS    string
	ClusterDomain string
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
	logrus.Info("Starting kubelet")
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
			"--kube-reserved-cgroup=system.slice",
			"--runtime-cgroups=/system.slice/containerd.service",
			"--kubelet-cgroups=/system.slice/containerd.service",
		},
	}

	err := retry.Do(func() error {
		kubeletconfig, err := k.KubeletConfigClient.Get()
		if err != nil {
			return err
		}

		err = ioutil.WriteFile(kubeletConfigPath, []byte(kubeletconfig), 0700)
		if err != nil {
			return errors.Wrap(err, "failed to write kubelet config to disk")
		}

		return nil

	})

	if err != nil {
		return err
	}

	k.supervisor.Supervise()

	return nil
}

// Stop stops kubelet
func (k *Kubelet) Stop() error {
	return k.supervisor.Stop()
}
