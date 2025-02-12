// SPDX-FileCopyrightText: 2023 k0s authors
// SPDX-License-Identifier: Apache-2.0

package containerd

import (
	"net/url"
	"path/filepath"
)

func Address(_ string) string {
	return `\\.\pipe\containerd-containerd`
}

func Endpoint(runDir string) *url.URL {
	return &url.URL{
		Scheme: "npipe",
		Path:   filepath.ToSlash(Address(runDir)),
	}
}
