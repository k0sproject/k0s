/*
Copyright 2023 k0s authors

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

package containerd

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"text/template"

	"github.com/BurntSushi/toml"
	"github.com/k0sproject/k0s/pkg/apis/k0s.k0sproject.io/v1beta1"
	"github.com/k0sproject/k0s/pkg/constant"
	"github.com/sirupsen/logrus"
)

// FIXME Figure out if we can get this programmatically from containerd so we're always in sync
const runcCRIConfigTemplate = `
[plugins."io.containerd.grpc.v1.cri"]
    device_ownership_from_security_context = false
    disable_apparmor = false
    disable_cgroup = false
    disable_hugetlb_controller = true
    disable_proc_mount = false
    disable_tcp_service = true
    enable_selinux = false
    enable_tls_streaming = false
    enable_unprivileged_icmp = false
    enable_unprivileged_ports = false
    ignore_image_defined_volumes = false
    max_concurrent_downloads = 3
    max_container_log_line_size = 16384
    netns_mounts_under_state_dir = false
    restrict_oom_score_adj = false
    sandbox_image = "{{.PauseImage}}"
    selinux_category_range = 1024
    stats_collect_period = 10
    stream_idle_timeout = "4h0m0s"
    stream_server_address = "127.0.0.1"
    stream_server_port = "0"
    systemd_cgroup = false
    tolerate_missing_hugetlb_controller = true
    unset_seccomp_profile = ""

    [plugins."io.containerd.grpc.v1.cri".cni]
      bin_dir = "/opt/cni/bin"
      conf_dir = "/etc/cni/net.d"
      conf_template = ""
      ip_pref = ""
      max_conf_num = 1

    [plugins."io.containerd.grpc.v1.cri".containerd]
      default_runtime_name = "runc"
      disable_snapshot_annotations = true
      discard_unpacked_layers = false
      ignore_rdt_not_enabled_errors = false
      no_pivot = false
      snapshotter = "overlayfs"

      [plugins."io.containerd.grpc.v1.cri".containerd.default_runtime]
        base_runtime_spec = ""
        cni_conf_dir = ""
        cni_max_conf_num = 0
        container_annotations = []
        pod_annotations = []
        privileged_without_host_devices = false
        runtime_engine = ""
        runtime_path = ""
        runtime_root = ""
        runtime_type = ""

        [plugins."io.containerd.grpc.v1.cri".containerd.default_runtime.options]

      [plugins."io.containerd.grpc.v1.cri".containerd.runtimes]

        [plugins."io.containerd.grpc.v1.cri".containerd.runtimes.runc]
          base_runtime_spec = ""
          cni_conf_dir = ""
          cni_max_conf_num = 0
          container_annotations = []
          pod_annotations = []
          privileged_without_host_devices = false
          runtime_engine = ""
          runtime_path = ""
          runtime_root = ""
          runtime_type = "io.containerd.runc.v2"

          [plugins."io.containerd.grpc.v1.cri".containerd.runtimes.runc.options]
            BinaryName = ""
            CriuImagePath = ""
            CriuPath = ""
            CriuWorkPath = ""
            IoGid = 0
            IoUid = 0
            NoNewKeyring = false
            NoPivotRoot = false
            Root = ""
            ShimCgroup = ""
            SystemdCgroup = false
`

const importsPath = "/etc/k0s/containerd.d/*.toml"
const containerdCRIConfigPath = "/run/k0s/containerd-cri.toml"

type CRIConfigurer struct {
	loadPath       string
	pauseImage     string
	criRuntimePath string

	log *logrus.Entry
}

func NewConfigurer() *CRIConfigurer {

	pauseImage := v1beta1.ImageSpec{
		Image:   constant.KubePauseContainerImage,
		Version: constant.KubePauseContainerImageVersion,
	}
	return &CRIConfigurer{
		loadPath:       importsPath,
		criRuntimePath: containerdCRIConfigPath,
		pauseImage:     pauseImage.URI(),
		log:            logrus.WithField("component", "containerd"),
	}
}

// Resolves imports from the import glob path
// If the partial config has CRI plugin enabled, it will add the runc CRI config (single file)
// if no CRI plugin is found, it will add the file as-is to imports
func (c *CRIConfigurer) ResolveImports() ([]string, error) {
	var imports []string
	var criConfigBuffer bytes.Buffer

	// Add runc CRI config
	err := c.generateRunCConfig(&criConfigBuffer)
	if err != nil {
		return nil, err
	}

	files, err := filepath.Glob(c.loadPath)
	c.log.Debugf("found containerd config files: %v", files)
	if err != nil {
		return nil, err
	}
	for _, file := range files {
		data, err := os.ReadFile(file)
		if err != nil {
			return nil, err
		}
		c.log.Debugf("parsing containerd config file: %s", file)
		hasCRI, err := c.hasCRIPluginConfig(data)
		if err != nil {
			return nil, err
		}
		if hasCRI {
			c.log.Debugf("found CRI plugin config in %s, appending to CRI config", file)
			// Append all CRI plugin configs into single file
			// and add it to imports
			criConfigBuffer.WriteString(fmt.Sprintf("# appended from %s\n", file))
			_, err := criConfigBuffer.Write(data)
			if err != nil {
				return nil, err
			}
		} else {
			c.log.Debugf("adding %s as-is to imports", file)
			// Add file to imports
			imports = append(imports, file)
		}
	}

	// Write the CRI config to a file and add it to imports
	err = os.WriteFile(c.criRuntimePath, criConfigBuffer.Bytes(), 0644)
	if err != nil {
		return nil, err
	}
	imports = append(imports, c.criRuntimePath)

	return imports, nil
}

func (c *CRIConfigurer) generateRunCConfig(w io.Writer) error {
	t, err := template.New("runcCRIConfig").Parse(runcCRIConfigTemplate)
	if err != nil {
		return err
	}

	data := struct {
		PauseImage string
	}{
		PauseImage: c.pauseImage,
	}

	err = t.Execute(w, data)
	if err != nil {
		return err
	}

	return nil
}

func (c *CRIConfigurer) hasCRIPluginConfig(data []byte) (bool, error) {
	var tomlConfig map[string]interface{}
	if err := toml.Unmarshal(data, &tomlConfig); err != nil {
		return false, err
	}
	c.log.Debugf("parsed containerd config: %+v", tomlConfig)
	if _, ok := tomlConfig["plugins"]; ok {
		if _, ok := tomlConfig["plugins"].(map[string]interface{})["io.containerd.grpc.v1.cri"]; ok {
			return true, nil
		}
	}
	return false, nil
}
