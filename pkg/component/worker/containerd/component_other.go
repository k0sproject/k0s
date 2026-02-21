//go:build !windows

// SPDX-FileCopyrightText: 2025 k0s authors
// SPDX-License-Identifier: Apache-2.0

package containerd

import (
	"github.com/k0sproject/k0s/pkg/assets"
)

const (
	defaultConfPath    = "/etc/k0s/containerd.toml"
	defaultImportsPath = "/etc/k0s/containerd.d/"
)

// containerRuntimeExecLabel is the SELinux label for container runtime executables.
// This label is required for containerd, runc, and related binaries to function
// correctly on SELinux-enforcing systems. The label allows these binaries to
// interact with container processes and manage container lifecycle.
const containerRuntimeExecLabel = "system_u:object_r:container_runtime_exec_t:s0"

var additionalExecutableNames = [...]string{
	"containerd-shim-runc-v2",
	"runc",
}

func stageExecutable(dir, name string) (string, error) {
	return assets.StageExecutable(dir, name, assets.WithSELinuxLabel(containerRuntimeExecLabel))
}
