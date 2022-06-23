/*
Copyright 2022 k0s authors

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
	"path/filepath"

	"github.com/sirupsen/logrus"

	"github.com/k0sproject/k0s/internal/pkg/dir"
	"github.com/k0sproject/k0s/internal/pkg/templatewriter"
	"github.com/k0sproject/k0s/pkg/apis/k0s.k0sproject.io/v1beta1"
	"github.com/k0sproject/k0s/pkg/component"
	"github.com/k0sproject/k0s/pkg/constant"
)

// DefaultPSP implements system RBAC reconciler
/* It always creates two sets of PSP rules:
	- 00-k0s-privileged: allows "privileged" stuff (host namespaces, privileged etc.) to be running
	- 99-k0s-restricted: more restricted rules, usually suitable for "normal" workloads
Depending on user config, we select either of the above rule sets to be the default
*/
type DefaultPSP struct {
	k0sVars        constant.CfgVars
	manifestDir    string
	previousPolicy string
}

var _ component.Component = (*DefaultPSP)(nil)
var _ component.ReconcilerComponent = (*DefaultPSP)(nil)

// NewDefaultPSP creates new system level RBAC reconciler
func NewDefaultPSP(k0sVars constant.CfgVars) *DefaultPSP {
	return &DefaultPSP{
		k0sVars:     k0sVars,
		manifestDir: path.Join(k0sVars.ManifestsDir, "defaultpsp"),
	}
}

// Init does currently nothing
func (d *DefaultPSP) Init(_ context.Context) error {
	err := dir.Init(d.manifestDir, constant.ManifestsDirMode)
	if err != nil {
		return err
	}
	return nil
}

// Run reconciles the k0s default PSP rules
func (d *DefaultPSP) Run(_ context.Context) error {
	return nil
}

// Stop does currently nothing
func (d *DefaultPSP) Stop() error {
	return nil
}

// Reconcile detects changes in configuration and applies them to the component
func (d *DefaultPSP) Reconcile(_ctx context.Context, clusterConfig *v1beta1.ClusterConfig) error {
	log := logrus.WithField("component", "DefaultPSP")
	log.Debug("reconcile method called for: DefaultPSP")
	if d.previousPolicy == clusterConfig.Spec.PodSecurityPolicy.DefaultPolicy {
		log.Debug("new PSP matches existing, no reconcile needed")
		return nil
	}
	log.Debugf("policy changed from %s to %s", d.previousPolicy, clusterConfig.Spec.PodSecurityPolicy.DefaultPolicy)
	tw := templatewriter.TemplateWriter{
		Name:     "default-psp",
		Template: defaultPSPTemplate,
		Data: struct{ DefaultPSP string }{
			DefaultPSP: clusterConfig.Spec.PodSecurityPolicy.DefaultPolicy,
		},
		Path: filepath.Join(d.manifestDir, "default-psp.yaml"),
	}
	err := tw.Write()
	if err != nil {
		return fmt.Errorf("error writing default PSP manifests, will NOT retry: %w", err)
	}
	d.previousPolicy = clusterConfig.Spec.PodSecurityPolicy.DefaultPolicy
	return nil
}

const defaultPSPTemplate = `
apiVersion: policy/v1beta1
kind: PodSecurityPolicy
metadata:
  name: 00-k0s-privileged
  annotations:
    kubernetes.io/description: 'privileged allows full unrestricted access to
      pod features, as if the PodSecurityPolicy controller was not enabled.'
    seccomp.security.alpha.kubernetes.io/allowedProfileNames: '*'
  labels:
    kubernetes.io/cluster-service: "true"
spec:
  privileged: true
  allowPrivilegeEscalation: true
  allowedCapabilities:
  - '*'
  volumes:
  - '*'
  hostNetwork: true
  hostPorts:
  - min: 0
    max: 65535
  hostIPC: true
  hostPID: true
  runAsUser:
    rule: 'RunAsAny'
  seLinux:
    rule: 'RunAsAny'
  supplementalGroups:
    rule: 'RunAsAny'
  fsGroup:
    rule: 'RunAsAny'
  readOnlyRootFilesystem: false
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: k0s:podsecuritypolicy:privileged
  labels:
    kubernetes.io/cluster-service: "true"
rules:
- apiGroups:
  - policy
  resourceNames:
  - 00-k0s-privileged
  resources:
  - podsecuritypolicies
  verbs:
  - use
---
apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  name: kube-system-psp
  namespace: kube-system
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: k0s:podsecuritypolicy:privileged
subjects:
# For the kubeadm kube-system nodes
- apiGroup: rbac.authorization.k8s.io
  kind: Group
  name: system:nodes
# For the cluster-admin role
- apiGroup: rbac.authorization.k8s.io
  kind: Group
  name: cluster-admin
# For all service accounts in the kube-system namespace
- apiGroup: rbac.authorization.k8s.io
  kind: Group
  name: system:serviceaccounts:kube-system
---
apiVersion: policy/v1beta1
kind: PodSecurityPolicy
metadata:
  annotations:
  name: 99-k0s-restricted
spec:
  allowedCapabilities: []  # default set of capabilities are implicitly allowed
  allowPrivilegeEscalation: false
  fsGroup:
    rule: 'MustRunAs'
    ranges:
      # Forbid adding the root group.
      - min: 1
        max: 65535
  hostIPC: false
  hostNetwork: false
  hostPID: false
  privileged: false
  readOnlyRootFilesystem: false
  runAsUser:
    rule: 'MustRunAsNonRoot'
  seLinux:
    rule: 'RunAsAny'
  supplementalGroups:
    rule: 'MustRunAs'
    ranges:
      # Forbid adding the root group.
      - min: 1
        max: 65535
  volumes:
  - 'configMap'
  - 'downwardAPI'
  - 'emptyDir'
  - 'persistentVolumeClaim'
  - 'projected'
  - 'secret'
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: k0s:podsecuritypolicy:default
rules:
- apiGroups:
  - policy
  resourceNames:
  - {{ .DefaultPSP }}
  resources:
  - podsecuritypolicies
  verbs:
  - use
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: default-psp
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: k0s:podsecuritypolicy:default
subjects:
# For authenticated users
- apiGroup: rbac.authorization.k8s.io
  kind: Group
  name: system:authenticated
# For all service accounts
- apiGroup: rbac.authorization.k8s.io
  kind: Group
  name: system:serviceaccounts
`

// Health-check interface
func (d *DefaultPSP) Healthy() error { return nil }
