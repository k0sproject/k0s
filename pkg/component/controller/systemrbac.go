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
	_ "embed"
	"path"
	"path/filepath"

	"github.com/k0sproject/k0s/internal/pkg/dir"
	"github.com/k0sproject/k0s/internal/pkg/file"
	"github.com/k0sproject/k0s/pkg/component/manager"
	"github.com/k0sproject/k0s/pkg/constant"
)

// SystemRBAC implements system RBAC reconciler
type SystemRBAC struct {
	manifestDir string
}

var _ manager.Component = (*SystemRBAC)(nil)

// NewSystemRBAC creates new system level RBAC reconciler
func NewSystemRBAC(manifestDir string) *SystemRBAC {
	return &SystemRBAC{manifestDir}
}

// Writes the bootstrap RBAC manifests into the manifests folder.
func (s *SystemRBAC) Init(context.Context) error {
	rbacDir := path.Join(s.manifestDir, "bootstraprbac")
	if err := dir.Init(rbacDir, constant.ManifestsDirMode); err != nil {
		return err
	}

	return file.WriteContentAtomically(filepath.Join(rbacDir, "bootstrap-rbac.yaml"), systemRBAC, 0644)
}

func (s *SystemRBAC) Start(context.Context) error { return nil }
func (s *SystemRBAC) Stop() error                 { return nil }

//go:embed systemrbac.yaml
var systemRBAC []byte
