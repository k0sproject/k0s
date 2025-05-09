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
	"fmt"
	"io/fs"
	"path"
	"path/filepath"

	"github.com/k0sproject/k0s/internal/pkg/dir"
	"github.com/k0sproject/k0s/internal/pkg/file"
	"github.com/k0sproject/k0s/pkg/component/manager"
	"github.com/k0sproject/k0s/pkg/constant"
	"github.com/k0sproject/k0s/static"
)

var _ manager.Component = (*CRD)(nil)

// CRD unpacks bundled CRD definitions to the filesystem
type CRD struct {
	bundle       string
	manifestsDir string

	crdOpts
}

type crdOpts struct {
	stackName, assetsDir string
}

type CRDOption func(*crdOpts)

// NewCRD build new CRD
func NewCRD(manifestsDir, bundle string, opts ...CRDOption) *CRD {
	var options crdOpts
	for _, opt := range opts {
		opt(&options)
	}

	if options.assetsDir == "" {
		options.stackName = bundle
		options.assetsDir = bundle
	}

	return &CRD{
		bundle:       bundle,
		manifestsDir: manifestsDir,
		crdOpts:      options,
	}
}

func WithStackName(stackName string) CRDOption {
	return func(opts *crdOpts) { opts.stackName = stackName }
}

func WithCRDAssetsDir(assetsDir string) CRDOption {
	return func(opts *crdOpts) { opts.assetsDir = assetsDir }
}

func (c CRD) Init(context.Context) error {
	return dir.Init(filepath.Join(c.manifestsDir, c.stackName), constant.ManifestsDirMode)
}

// Run unpacks manifests from bindata
func (c CRD) Start(context.Context) error {
	crds, err := fs.ReadDir(static.CRDs, c.assetsDir)
	if err != nil {
		return fmt.Errorf("can't unbundle CRD `%s` manifests: %w", c.bundle, err)
	}

	for _, entry := range crds {
		filename := entry.Name()
		src := path.Join(c.assetsDir, filename)
		dst := filepath.Join(c.manifestsDir, c.stackName, fmt.Sprintf("%s-crd-%s", c.bundle, filename))

		content, err := fs.ReadFile(static.CRDs, src)
		if err != nil {
			return fmt.Errorf("failed to fetch CRD %s manifest %s: %w", c.bundle, filename, err)
		}
		if err := file.AtomicWithTarget(dst).
			WithPermissions(constant.CertMode).
			Write(content); err != nil {
			return fmt.Errorf("failed to save CRD %s manifest %s to FS: %w", c.bundle, filename, err)
		}
	}

	return nil
}

func (c CRD) Stop() error {
	return nil
}
