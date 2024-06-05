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
	"path"

	"github.com/k0sproject/k0s/pkg/component/manager"
	"github.com/k0sproject/k0s/static"
)

var _ manager.Component = (*CRD)(nil)

// CRD unpacks bundled CRD definitions to the filesystem
type CRD struct {
	saver  manifestsSaver
	bundle string

	crdOpts
}

type crdOpts struct {
	assetsDir string
}

type CRDOption func(*crdOpts)

// NewCRD build new CRD
func NewCRD(s manifestsSaver, bundle string, opts ...CRDOption) *CRD {
	var options crdOpts
	for _, opt := range opts {
		opt(&options)
	}

	if options.assetsDir == "" {
		options.assetsDir = bundle
	}

	return &CRD{
		saver:   s,
		bundle:  bundle,
		crdOpts: options,
	}
}

func WithCRDAssetsDir(assetsDir string) CRDOption {
	return func(opts *crdOpts) { opts.assetsDir = assetsDir }
}

// Init  (c CRD) Init(_ context.Context) error {
func (c CRD) Init(_ context.Context) error {
	return nil
}

// Run unpacks manifests from bindata
func (c CRD) Start(_ context.Context) error {
	crdAssetsPath := path.Join("manifests", c.assetsDir, "CustomResourceDefinition")
	crds, err := static.AssetDir(crdAssetsPath)
	if err != nil {
		return fmt.Errorf("can't unbundle CRD `%s` manifests: %w", c.bundle, err)
	}

	for _, filename := range crds {
		manifestName := fmt.Sprintf("%s-crd-%s", c.bundle, filename)
		content, err := static.Asset(path.Join(crdAssetsPath, filename))
		if err != nil {
			return fmt.Errorf("failed to fetch crd `%s`: %w", filename, err)
		}
		if err := c.saver.Save(manifestName, content); err != nil {
			return fmt.Errorf("failed to save CRD `%s` manifest `%s` to FS: %w", c.bundle, manifestName, err)
		}
	}

	return nil
}

func (c CRD) Stop() error {
	return nil
}
