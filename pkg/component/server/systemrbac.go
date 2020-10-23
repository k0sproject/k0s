/*
Copyright 2020 Mirantis, Inc.

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
package server

import (
	"os"
	"path"
	"path/filepath"

	config "github.com/Mirantis/mke/pkg/apis/v1beta1"
	"github.com/Mirantis/mke/pkg/constant"
	"github.com/Mirantis/mke/pkg/util"
	"github.com/pkg/errors"
)

// SystemRBAC implements system RBAC reconciler
type SystemRBAC struct {
}

// NewSystemRBAC creates new system level RBAC reconciler
func NewSystemRBAC(clusterSpec *config.ClusterSpec) (*SystemRBAC, error) {

	return &SystemRBAC{}, nil
}

// Init does nothing
func (s *SystemRBAC) Init() error {
	return nil
}

// Run reconciles the mke related system RBAC rules
func (s *SystemRBAC) Run() error {
	rbacDir := path.Join(constant.ManifestsDir, "bootstraprbac")
	err := os.MkdirAll(rbacDir, constant.ManifestsDirMode)
	if err != nil {
		return err
	}
	tw := util.TemplateWriter{
		Name:     "bootstrap-rbac",
		Template: bootstrapRBACTemplate,
		Data:     struct{}{},
		Path:     filepath.Join(rbacDir, "bootstrap-rbac.yaml"),
	}
	err = tw.Write()
	if err != nil {
		return errors.Wrap(err, "error writing bootstrap-rbac manifests, will NOT retry")

	}
	return nil
}

// Stop does currently nothing
func (s *SystemRBAC) Stop() error {
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
