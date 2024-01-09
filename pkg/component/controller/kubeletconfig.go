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
	"context"
	"errors"
	"os"
	"path/filepath"

	"github.com/k0sproject/k0s/internal/pkg/dir"
	"github.com/k0sproject/k0s/internal/pkg/file"
	"github.com/k0sproject/k0s/pkg/applier"
	"github.com/k0sproject/k0s/pkg/component/manager"
	"github.com/k0sproject/k0s/pkg/config"
	"github.com/k0sproject/k0s/pkg/constant"

	"github.com/sirupsen/logrus"
)

// Dummy checks so we catch easily if we miss some interface implementation
var _ manager.Component = (*KubeletConfig)(nil)

// KubeletConfig is the old, replaced reconciler for generic kubelet configs.
type KubeletConfig struct {
	k0sVars *config.CfgVars
}

// NewKubeletConfig creates a new KubeletConfig reconciler that merely
// uninstalls itself, if it still exists.
func NewKubeletConfig(k0sVars *config.CfgVars) *KubeletConfig {
	return &KubeletConfig{k0sVars}
}

func (k *KubeletConfig) Init(context.Context) error {
	kubeletDir := filepath.Join(k.k0sVars.ManifestsDir, "kubelet")

	err := dir.Init(kubeletDir, constant.ManifestsDirMode)
	if err != nil {
		return err
	}

	var errs []error

	// Iterate over all files that would be read by the manifest applier and
	// rename them, so they won't be applied anymore.
	if manifests, err := applier.FindManifestFilesInDir(kubeletDir); err != nil {
		errs = append(errs, err)
	} else {
		for _, manifest := range manifests {
			// Reserve a new unique file name to preserve the file's contents.
			f, err := os.CreateTemp(filepath.Dir(manifest), filepath.Base(manifest)+".*.removed")
			if err != nil {
				errs = append(errs, err)
				continue
			}
			errs = append(errs, f.Close())

			// Rename the file, overwriting the target.
			errs = append(errs, os.Rename(manifest, f.Name()))
		}
	}

	const removalNotice = `The kubelet-config component has been replaced by the worker-config component in k0s 1.26.
It has been removed in k0s 1.29.
`

	errs = append(errs, file.WriteContentAtomically(filepath.Join(kubeletDir, "removed.txt"), []byte(removalNotice), constant.CertMode))

	// Remove a potential deprecation notice
	if err := os.Remove(filepath.Join(kubeletDir, "deprecated.txt")); err != nil && !errors.Is(err, os.ErrNotExist) {
		logrus.WithField("component", "kubelet-config").WithError(err).Warn("Failed to delete deprecation notice")
	}

	return errors.Join(errs...)
}

func (k *KubeletConfig) Start(context.Context) error { return nil }
func (k *KubeletConfig) Stop() error                 { return nil }
