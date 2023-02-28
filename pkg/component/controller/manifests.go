/*
Copyright 2020 k0s authors

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

package controller

import (
	"crypto/md5"
	"fmt"
	"path/filepath"

	"github.com/k0sproject/k0s/internal/pkg/dir"
	"github.com/k0sproject/k0s/internal/pkg/file"
	"github.com/k0sproject/k0s/pkg/constant"
	"github.com/sirupsen/logrus"
)

// FsManifestsSaver saves all given manifests under the specified root dir
type FsManifestsSaver struct {
	dir string
}

// Save saves given manifest under the given path
func (f FsManifestsSaver) Save(dst string, content []byte) error {
	target := filepath.Join(f.dir, dst)

	if err := file.WriteContentAtomically(target, content, constant.CertMode); err != nil {
		return err
	}

	logrus.WithField("component", "manifest-saver").Debugf("Successfully wrote %s:%s", target, hash(content))
	return nil
}

func hash(data []byte) string {
	return fmt.Sprintf("%x", md5.Sum(data))
}

// NewManifestsSaver builds new filesystem manifests saver
func NewManifestsSaver(manifest string, dataDir string) (*FsManifestsSaver, error) {
	manifestDir := filepath.Join(dataDir, "manifests", manifest)
	err := dir.Init(manifestDir, constant.ManifestsDirMode)
	if err != nil {
		return nil, err
	}
	return &FsManifestsSaver{dir: manifestDir}, nil
}
