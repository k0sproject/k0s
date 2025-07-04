// SPDX-FileCopyrightText: 2020 k0s authors
// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"context"
	"fmt"
	"io/fs"
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
func (c CRD) Start(context.Context) error {
	crds, err := fs.ReadDir(static.CRDs, c.assetsDir)
	if err != nil {
		return fmt.Errorf("can't unbundle CRD `%s` manifests: %w", c.bundle, err)
	}

	for _, entry := range crds {
		filename := entry.Name()
		manifestName := fmt.Sprintf("%s-crd-%s", c.bundle, filename)
		content, err := fs.ReadFile(static.CRDs, path.Join(c.assetsDir, filename))
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
