// SPDX-FileCopyrightText: 2020 k0s authors
// SPDX-License-Identifier: Apache-2.0

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
