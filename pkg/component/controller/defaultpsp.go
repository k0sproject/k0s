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
	"github.com/k0sproject/k0s/pkg/apis/k0s.k0sproject.io/v1beta1"
	"github.com/k0sproject/k0s/pkg/constant"
)

// DefaultPSP implements system RBAC reconciler
/* It always creates two sets of PSP rules:
	- 00-k0s-privileged: allows "privileged" stuff (host namespaces, privileged etc.) to be running
	- 99-k0s-restricted: more restricted rules, usually suitable for "normal" workloads
Depending on user config, we select either of the above rule sets to be the default
*/
type DefaultPSP struct {
	clusterSpec *v1beta1.ClusterSpec
	k0sVars     constant.CfgVars
}

// NewDefaultPSP creates new system level RBAC reconciler
func NewDefaultPSP(clusterSpec *v1beta1.ClusterSpec, k0sVars constant.CfgVars) (*DefaultPSP, error) {
	return &DefaultPSP{
		clusterSpec: clusterSpec,
		k0sVars:     k0sVars,
	}, nil
}

// Init does currently nothing
func (d *DefaultPSP) Init() error {
	return nil
}

// Run reconciles the k0s default PSP rules
func (d *DefaultPSP) Run() error {
	pspDir := path.Join(d.k0sVars.ManifestsDir, "defaultpsp")
	err := dir.Init(pspDir, constant.ManifestsDirMode)
	if err != nil {
		return err
	}
	tw := templatewriter.TemplateWriter{
		Name:     "default-psp",
		Template: defaultPSPTemplate,
		Data: struct{ DefaultPSP string }{
			DefaultPSP: d.clusterSpec.PodSecurityPolicy.DefaultPolicy,
		},
		Path: filepath.Join(pspDir, "default-psp.yaml"),
	}
	err = tw.Write()
	if err != nil {
		return fmt.Errorf("error writing default PSP manifests, will NOT retry: %w", err)
	}
	return nil
}

// Stop does currently nothing
func (d *DefaultPSP) Stop() error {
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
    addonmanager.kubernetes.io/mode: Reconcile
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
    addonmanager.kubernetes.io/mode: Reconcile
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
    rule: 'RunAsNonRoot'
  supplementalGroups:
    rule: 'RunAsNonRoot'
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
  hostNetwork: false
  runAsUser:
    rule: 'RunAsAny'
  seLinux:
    rule: 'RunAsAny'
  supplementalGroups:
    rule: 'RunAsAny'
  fsGroup:
    rule: 'RunAsAny'
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
