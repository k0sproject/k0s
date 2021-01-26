// +build windows

/*
Copyright 2020 Mirantis, Inc.

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
package worker

import (
	"github.com/sirupsen/logrus"
	"golang.org/x/sync/errgroup"

	"github.com/k0sproject/k0s/internal/util"
	"github.com/k0sproject/k0s/pkg/assets"
	"github.com/k0sproject/k0s/pkg/constant"
	"github.com/k0sproject/k0s/pkg/supervisor"
)

// ContainerD implement the component interface to manage containerd as k0s component
type ContainerD struct {
	supervisor supervisor.Supervisor
	LogLevel   string
	K0sVars    constant.CfgVars
}

// Init extracts the needed binaries
func (c *ContainerD) Init() error {
	g := new(errgroup.Group)
	for _, bin := range []string{"containerd.exe", "containerd-shim-runhcs-v1.exe", "ctr.exe"} {
		b := bin
		g.Go(func() error {
			return assets.Stage(c.K0sVars.BinDir, b, constant.BinDirMode)
		})
	}

	return g.Wait()
}

// Run runs containerD
func (c *ContainerD) Run() error {
	logrus.Info("Starting containerD for windows")
	args := util.MappedArgs{
		"--log-level": c.LogLevel,
		// TODO Do we need any other stuff???
	}

	tw := util.TemplateWriter{
		Name:     "container-config",
		Template: configTemplate,
		Data:     struct{}{},
		Path:     "C:\\Program Files\\containerd\\config.toml",
	}

	err := tw.Write()
	if err != nil {
		return err
	}

	c.supervisor = supervisor.Supervisor{
		Name:    "containerd",
		BinPath: assets.BinPath("containerd.exe", c.K0sVars.BinDir),
		RunDir:  c.K0sVars.RunDir,
		DataDir: c.K0sVars.DataDir,
		Args:    args.ToArgs(),
	}

	c.supervisor.Supervise()

	return nil
}

// Stop stops containerD
func (c *ContainerD) Stop() error {
	return c.supervisor.Stop()
}

// Health-check interface
func (c *ContainerD) Healthy() error { return nil }

const configTemplate = `
version = 2
root = "C:\\ProgramData\\containerd\\root"
state = "C:\\ProgramData\\containerd\\state"
plugin_dir = ""
disabled_plugins = []
required_plugins = []
oom_score = 0

[grpc]
  address = "\\\\.\\pipe\\containerd-containerd"
  tcp_address = ""
  tcp_tls_cert = ""
  tcp_tls_key = ""
  uid = 0
  gid = 0
  max_recv_message_size = 16777216
  max_send_message_size = 16777216

[ttrpc]
  address = ""
  uid = 0
  gid = 0

[debug]
  address = ""
  uid = 0
  gid = 0
  level = ""

[metrics]
  address = ""
  grpc_histogram = false

[cgroup]
  path = ""

[timeouts]
  "io.containerd.timeout.shim.cleanup" = "5s"
  "io.containerd.timeout.shim.load" = "5s"
  "io.containerd.timeout.shim.shutdown" = "3s"
  "io.containerd.timeout.task.state" = "2s"

[plugins]
  [plugins."io.containerd.gc.v1.scheduler"]
    pause_threshold = 0.02
    deletion_threshold = 0
    mutation_threshold = 100
    schedule_delay = "0s"
    startup_delay = "100ms"
  [plugins."io.containerd.grpc.v1.cri"]
    disable_tcp_service = true
    stream_server_address = "127.0.0.1"
    stream_server_port = "0"
    stream_idle_timeout = "4h0m0s"
    enable_selinux = false
    selinux_category_range = 0
    sandbox_image = "mcr.microsoft.com/oss/kubernetes/pause:1.4.1"
    stats_collect_period = 10
    systemd_cgroup = false
    enable_tls_streaming = false
    max_container_log_line_size = 16384
    disable_cgroup = false
    disable_apparmor = false
    restrict_oom_score_adj = false
    max_concurrent_downloads = 3
    disable_proc_mount = false
    unset_seccomp_profile = ""
    tolerate_missing_hugetlb_controller = false
    disable_hugetlb_controller = false
    ignore_image_defined_volumes = false
    [plugins."io.containerd.grpc.v1.cri".containerd]
      snapshotter = "windows"
      default_runtime_name = "runhcs-wcow-process"
      no_pivot = false
      disable_snapshot_annotations = false
      discard_unpacked_layers = false
      [plugins."io.containerd.grpc.v1.cri".containerd.default_runtime]
        runtime_type = "io.containerd.runhcs.v1"
        runtime_engine = ""
        runtime_root = ""
        privileged_without_host_devices = false
        base_runtime_spec = ""
      [plugins."io.containerd.grpc.v1.cri".containerd.untrusted_workload_runtime]
        runtime_type = ""
        runtime_engine = ""
        runtime_root = ""
        privileged_without_host_devices = false
        base_runtime_spec = ""
      [plugins."io.containerd.grpc.v1.cri".containerd.runtimes]
        [plugins."io.containerd.grpc.v1.cri".containerd.runtimes.runhcs-wcow-process]
          runtime_type = "io.containerd.runhcs.v1"
          runtime_engine = ""
          runtime_root = ""
          privileged_without_host_devices = false
          base_runtime_spec = ""
    [plugins."io.containerd.grpc.v1.cri".cni]
      bin_dir = "C:\\k\\cni"
      conf_dir = "C:\\k\\cni\\config"
      max_conf_num = 1
      conf_template = ""
    [plugins."io.containerd.grpc.v1.cri".registry]
      [plugins."io.containerd.grpc.v1.cri".registry.mirrors]
        [plugins."io.containerd.grpc.v1.cri".registry.mirrors."docker.io"]
          endpoint = ["https://registry-1.docker.io"]
    [plugins."io.containerd.grpc.v1.cri".image_decryption]
      key_model = ""
    [plugins."io.containerd.grpc.v1.cri".x509_key_pair_streaming]
      tls_cert_file = ""
      tls_key_file = ""
  [plugins."io.containerd.internal.v1.opt"]
    path = "C:\\ProgramData\\containerd\\root\\opt"
  [plugins."io.containerd.internal.v1.restart"]
    interval = "10s"
  [plugins."io.containerd.metadata.v1.bolt"]
    content_sharing_policy = "shared"
  [plugins."io.containerd.runtime.v2.task"]
    platforms = ["windows/amd64", "linux/amd64"]
  [plugins."io.containerd.service.v1.diff-service"]
    default = ["windows", "windows-lcow"]
`
