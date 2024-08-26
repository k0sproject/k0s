/*
Copyright 2024 k0s authors

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
