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
	"path"
	"path/filepath"

	"github.com/k0sproject/k0s/internal/pkg/dir"
	"github.com/k0sproject/k0s/internal/pkg/templatewriter"
	"github.com/k0sproject/k0s/pkg/constant"
	"github.com/sirupsen/logrus"
)

// SystemRBAC implements system RBAC reconciler
type SystemRBAC struct {
	manifestDir string
}

// NewSystemRBAC creates new system level RBAC reconciler
func NewSystemRBAC(manifestDir string) (*SystemRBAC, error) {
	return &SystemRBAC{
		manifestDir: manifestDir,
	}, nil
}

// Init does nothing
func (s *SystemRBAC) Init() error {
	return nil
}

// Run reconciles the k0s related system RBAC rules
func (s *SystemRBAC) Run() error {
	rbacDir := path.Join(s.manifestDir, "bootstraprbac")
	err := dir.Init(rbacDir, constant.ManifestsDirMode)
	if err != nil {
		return err
	}
	tw := templatewriter.TemplateWriter{
		Name:     "bootstrap-rbac",
		Template: bootstrapRBACTemplate,
		Data:     struct{}{},
		Path:     filepath.Join(rbacDir, "bootstrap-rbac.yaml"),
	}
	err = tw.Write()
	if err != nil {
		return fmt.Errorf("error writing bootstrap-rbac manifests, will NOT retry: %w", err)
	}
	return nil
}

// Stop does currently nothing
func (s *SystemRBAC) Stop() error {
	return nil
}

// Reconcile detects changes in configuration and applies them to the component
func (s *SystemRBAC) Reconcile() error {
	logrus.Debug("reconcile method called for: SystemRBAC")
	return nil
}

const bootstrapRBACTemplate = `
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: kubelet-bootstrap
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: system:node-bootstrapper
subjects:
- apiGroup: rbac.authorization.k8s.io
  kind: Group
  name: system:bootstrappers
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: node-autoapprove-bootstrap
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: system:certificates.k8s.io:certificatesigningrequests:nodeclient
subjects:
- apiGroup: rbac.authorization.k8s.io
  kind: Group
  name: system:bootstrappers
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: node-autoapprove-certificate-rotation
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: system:certificates.k8s.io:certificatesigningrequests:selfnodeclient
subjects:
- apiGroup: rbac.authorization.k8s.io
  kind: Group
  name: system:nodes
`

// Health-check interface
func (s *SystemRBAC) Healthy() error { return nil }
