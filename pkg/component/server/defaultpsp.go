package server

import (
	"path/filepath"

	config "github.com/Mirantis/mke/pkg/apis/v1beta1"
	"github.com/Mirantis/mke/pkg/constant"
	"github.com/Mirantis/mke/pkg/util"
	"github.com/pkg/errors"
)

// DefaultPSP implements system RBAC reconciler
/* It always creates two sets of PSP rules:
	- 00-mke-privileged: allows "privileged" stuff (host namespaces, privileged etc.) to be running
	- 99-mke-restricted: more restricted rules, usually suitable for "normal" workloads
Depending on user config, we select either of the above rule sets to be the default
*/
type DefaultPSP struct {
	clusterSpec *config.ClusterSpec
}

// NewDefaultPSP creates new system level RBAC reconciler
func NewDefaultPSP(clusterSpec *config.ClusterSpec) (*DefaultPSP, error) {
	return &DefaultPSP{
		clusterSpec: clusterSpec,
	}, nil
}

// Init does currently nothing
func (d *DefaultPSP) Init() error {
	return nil
}

// Run reconciles the mke default PSP rules
func (d *DefaultPSP) Run() error {
	tw := util.TemplateWriter{
		Name:     "default-psp",
		Template: defaultPSPTemplate,
		Data: struct{ DefaultPSP string }{
			DefaultPSP: d.clusterSpec.PodSecurityPolicy.DefaultPolicy,
		},
		Path: filepath.Join(constant.DataDir, "manifests", "defaultpsp", "default-psp.yaml"),
	}
	err := tw.Write()
	if err != nil {
		return errors.Wrap(err, "error writing default PSP manifests, will NOT retry")
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
  name: 00-mke-privileged
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
  name: mke:podsecuritypolicy:privileged
  labels:
    kubernetes.io/cluster-service: "true"
    addonmanager.kubernetes.io/mode: Reconcile
rules:
- apiGroups:
  - policy
  resourceNames:
  - 00-mke-privileged
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
  name: mke:podsecuritypolicy:privileged
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
  name: 99-mke-restricted
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
  name: mke:podsecuritypolicy:default
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
  name: mke:podsecuritypolicy:default
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
