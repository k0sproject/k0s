//go:build !windows

// SPDX-FileCopyrightText: 2023 k0s authors
// SPDX-License-Identifier: Apache-2.0

package containerd

import (
	"net/url"
	"path/filepath"
)

func Address(runDir string) string {
	return filepath.Join(runDir, "containerd.sock")
}

func Endpoint(runDir string) *url.URL {
	return &url.URL{
		Scheme: "unix",
		Path:   filepath.ToSlash(Address(runDir)),
	}
}
