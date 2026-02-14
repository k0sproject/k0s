// SPDX-FileCopyrightText: 2024 k0s authors
// SPDX-License-Identifier: Apache-2.0

package static

import (
	"embed"
	"io/fs"
	"path"
)

var (
	//go:embed _crds
	crds embed.FS
	CRDs fs.FS = subFS(crds, "_crds")
)

var (
	//go:embed manifests/calico
	calicoManifests embed.FS
	CalicoManifests fs.FS = subFS(calicoManifests, "manifests", "calico")
)

var (
	//go:embed manifests/windows
	windowsManifests embed.FS
	WindowsManifests fs.FS = subFS(windowsManifests, "manifests", "windows")
)

func subFS(f fs.FS, segments ...string) fs.FS {
	f, err := fs.Sub(f, path.Join(segments...))
	if err != nil {
		panic(err)
	}
	return f
}
