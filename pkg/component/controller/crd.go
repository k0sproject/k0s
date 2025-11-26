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

	"github.com/k0sproject/k0s/pkg/applier"
	"github.com/k0sproject/k0s/pkg/component/manager"
	"github.com/k0sproject/k0s/pkg/kubernetes"
	"github.com/k0sproject/k0s/static"
)

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
