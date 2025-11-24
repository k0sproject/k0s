// SPDX-FileCopyrightText: 2020 k0s authors
// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"bytes"
	"context"
	_ "embed"
	"fmt"
	"io"

	"github.com/k0sproject/k0s/pkg/applier"
	"github.com/k0sproject/k0s/pkg/component/manager"
	"github.com/k0sproject/k0s/pkg/kubernetes"
)

const SystemRBACStackName = "bootstraprbac"

// SystemRBAC implements system RBAC reconciler
type SystemRBAC struct {
	Clients          kubernetes.ClientFactoryInterface
	ExcludeAutopilot bool
}

var _ manager.Component = (*SystemRBAC)(nil)

// Applies the system RBAC manifests to the cluster.
func (s *SystemRBAC) Init(ctx context.Context) error {
	in := io.Reader(bytes.NewReader(systemRBAC))
	if !s.ExcludeAutopilot {
		in = io.MultiReader(in, bytes.NewReader(apSystemRBAC))
	}

	if err := applier.ApplyStack(ctx, s.Clients, in, SystemRBACStackName, SystemRBACStackName); err != nil {
		return fmt.Errorf("failed to apply system RBAC stack: %w", err)
	}

	return nil
}

func (s *SystemRBAC) Start(context.Context) error { return nil }
func (s *SystemRBAC) Stop() error                 { return nil }

//go:embed systemrbac.yaml
var systemRBAC []byte

//go:embed systemrbac-ap.yaml
var apSystemRBAC []byte
