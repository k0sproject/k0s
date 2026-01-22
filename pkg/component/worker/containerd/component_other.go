//go:build !windows

// SPDX-FileCopyrightText: 2025 k0s authors
// SPDX-License-Identifier: Apache-2.0

package containerd

import "github.com/k0sproject/k0s/pkg/assets"

const (
	defaultConfPath    = "/etc/k0s/containerd.toml"
	defaultImportsPath = "/etc/k0s/containerd.d/"
)

var additionalExecutableNames = [...]string{
	"containerd-shim-runc-v2",
	"runc",
}

func stageExecutable(dir, name string) (string, error) {
	return assets.StageExecutable(dir, name)
}
