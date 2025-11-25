// SPDX-FileCopyrightText: 2020 k0s authors
// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"bytes"
	"context"
	"fmt"
	"io/fs"
	"iter"
	"path"
	"path/filepath"

	"github.com/k0sproject/k0s/internal/pkg/dir"
	"github.com/k0sproject/k0s/internal/pkg/file"
	"github.com/k0sproject/k0s/pkg/applier"
	"github.com/k0sproject/k0s/pkg/component/manager"
	"github.com/k0sproject/k0s/pkg/constant"
	"github.com/k0sproject/k0s/pkg/kubernetes"
	"github.com/k0sproject/k0s/static"
)

var _ manager.Component = (*CRD)(nil)

// CRD unpacks bundled CRD definitions to the filesystem
//
// Deprecated: Use [CRDStack] instead.
type CRD struct {
	bundle       string
	manifestsDir string

	crdOpts
}

// CRDStack applies bundled CRDs.
type CRDStack struct {
	clients kubernetes.ClientFactoryInterface
	bundle  string

	crdOpts
}

type crdOpts struct {
	stackName, assetsDir string
}

type CRDOption func(*crdOpts)

// NewCRD build new CRD
//
// Deprecated: Use [NewCRDStack] instead.
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

var _ manager.Component = (*CRDStack)(nil)

// Creates a new CRD stack for the given bundle.
func NewCRDStack(clients kubernetes.ClientFactoryInterface, bundle string, opts ...CRDOption) *CRDStack {
	var options crdOpts
	for _, opt := range opts {
		opt(&options)
	}

	if options.assetsDir == "" {
		options.stackName = bundle
		options.assetsDir = bundle
	}

	return &CRDStack{
		clients: clients,
		bundle:  bundle,
		crdOpts: options,
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

// Applies this CRD stack. Implements [manager.Component].
func (c *CRDStack) Init(ctx context.Context) error {
	var crds bytes.Buffer
	for content, err := range eachCRD(c.assetsDir) {
		if err != nil {
			return err
		}

		crds.WriteString("\n---\n")
		crds.Write(content)
	}

	if err := applier.ApplyStack(ctx, c.clients, &crds, c.bundle, c.stackName); err != nil {
		return fmt.Errorf("failed to apply %s CRD stack: %w", c.bundle, err)
	}

	return nil
}

// Start implements [manager.Component]. It does nothing.
func (c *CRDStack) Start(context.Context) error {
	return nil
}

// Stop implements [manager.Component]. It does nothing.
func (*CRDStack) Stop() error {
	return nil
}

// Iterates over the contents of each CRD in the given asset directory.
func eachCRD(assetsDir string) iter.Seq2[[]byte, error] {
	return func(yield func([]byte, error) bool) {
		crdFiles, err := fs.ReadDir(static.CRDs, assetsDir)
		if err != nil {
			yield(nil, fmt.Errorf("failed to read %s CRD stack: %w", assetsDir, err))
			return
		}

		for _, entry := range crdFiles {
			filename := entry.Name()
			content, err := fs.ReadFile(static.CRDs, path.Join(assetsDir, filename))
			if err != nil {
				yield(nil, fmt.Errorf("failed to fetch %s CRD manifest %s: %w", assetsDir, filename, err))
				return
			}
			if !yield(content, nil) {
				return
			}
		}
	}
}
