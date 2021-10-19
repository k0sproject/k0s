/*
Copyright 2021 k0s authors

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
	"fmt"
	"os"
	"path"
	"path/filepath"

	"github.com/k0sproject/k0s/internal/util"
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
	if err := os.WriteFile(target, content, constant.ManifestsDirMode); err != nil {
		return fmt.Errorf("can't write manifest %s: %v", target, err)
	}
	logrus.WithField("component", "manifest-saver").Debugf("succesfully wrote %s", target)
	return nil
}

// NewManifestsSaver builds new filesystem manifests saver
func NewManifestsSaver(dir string, dataDir string) (*FsManifestsSaver, error) {
	manifestDir := path.Join(dataDir, "manifests", dir)
	err := util.InitDirectory(manifestDir, constant.ManifestsDirMode)
	if err != nil {
		return nil, err
	}
	return &FsManifestsSaver{dir: manifestDir}, nil
}
