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
	"bytes"
	"fmt"

	"github.com/k0sproject/k0s/internal/pkg/templatewriter"
	"github.com/k0sproject/k0s/pkg/apis/k0s.k0sproject.io/v1beta1"
	"github.com/k0sproject/k0s/pkg/component"
	"github.com/k0sproject/k0s/pkg/constant"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

// KubeRouter implements the kube-router reconciler component
type KubeRouter struct {
	log *logrus.Entry

	saver          manifestsSaver
	previousConfig kubeRouterConfig
	k0sVars        constant.CfgVars
}

var _ component.Component = &KubeRouter{}
var _ component.ReconcilerComponent = &KubeRouter{}

type kubeRouterConfig struct {
	MTU               int
	AutoMTU           bool
	CNIInstallerImage string
	CNIImage          string
	PeerRouterIPs     string
	PeerRouterASNs    string
	PullPolicy        string
}

// NewKubeRouter creates new KubeRouter reconciler component
func NewKubeRouter(k0sVars constant.CfgVars, manifestsSaver manifestsSaver) (*KubeRouter, error) {
	log := logrus.WithFields(logrus.Fields{"component": "kube-router"})
	return &KubeRouter{
		saver:   manifestsSaver,
		log:     log,
		k0sVars: k0sVars,
	}, nil
}

// Init does nothing
func (k *KubeRouter) Init() error { return nil }

// Healthy is a no-op check
func (k *KubeRouter) Healthy() error { return nil }

// Stop no-op as nothing running
func (k *KubeRouter) Stop() error { return nil }

// Reconcile detects changes in configuration and applies them to the component
func (k *KubeRouter) Reconcile(clusterConfig *v1beta1.ClusterConfig) error {
	logrus.Debug("reconcile method called for: KubeRouter")
	if clusterConfig.Spec.Network.Provider != constant.CNIProviderKubeRouter {
		return nil
	}

	existingCNI := existingCNIProvider(k.k0sVars.ManifestsDir)
	if existingCNI != "" && existingCNI != constant.CNIProviderKubeRouter {
		return fmt.Errorf("cannot change CNI provider from %s to %s", existingCNI, constant.CNIProviderKubeRouter)
	}

	cfg := kubeRouterConfig{
		AutoMTU:           clusterConfig.Spec.Network.KubeRouter.AutoMTU,
		MTU:               clusterConfig.Spec.Network.KubeRouter.MTU,
		PeerRouterIPs:     clusterConfig.Spec.Network.KubeRouter.PeerRouterIPs,
		PeerRouterASNs:    clusterConfig.Spec.Network.KubeRouter.PeerRouterASNs,
		CNIImage:          clusterConfig.Spec.Images.KubeRouter.CNI.URI(),
		CNIInstallerImage: clusterConfig.Spec.Images.KubeRouter.CNIInstaller.URI(),
		PullPolicy:        clusterConfig.Spec.Images.DefaultPullPolicy,
	}

	if cfg == k.previousConfig {
		k.log.Info("config matches with previous, not reconciling anything")
		return nil
	}

	output := bytes.NewBuffer([]byte{})
	tw := templatewriter.TemplateWriter{
		Name:     "kube-router",
		Template: kubeRouterTemplate,
		Data:     cfg,
	}

	err := tw.WriteToBuffer(output)
	if err != nil {
		return errors.Wrap(err, "error writing kube-router manifests, will NOT retry")
	}

	err = k.saver.Save("kube-router.yaml", output.Bytes())
	if err != nil {
		return errors.Wrap(err, "error writing kube-router manifests, will NOT retry")
	}
	return nil
}

// Run runs the kube-router reconciler
func (k *KubeRouter) Run() error {
	k.log.Info("starting to dump manifests")

	return nil
}

const kubeRouterTemplate = `---
apiVersion: v1
kind: ConfigMap
metadata:
  name: kube-router-cfg
  namespace: kube-system
  labels:
    tier: node
    k8s-app: kube-router
data:
  cni-conf.json: |
    {
       "cniVersion":"0.3.0",
       "name":"mynet",
       "plugins":[
          {
             "name":"kubernetes",
             "type":"bridge",
             {{- if not .AutoMTU }}
             "mtu": {{ .MTU }},
             {{- end }}
             "auto-mtu": {{ .AutoMTU }},
             "bridge":"kube-bridge",
             "isDefaultGateway":true,
             "ipam":{
                "type":"host-local"
             }
          },
          {
            "type":"portmap",
            "capabilities":{
               "snat":true,
               "portMappings":true
            }
         }
       ]
    }
---
apiVersion: apps/v1
kind: DaemonSet
metadata:
  labels:
    k8s-app: kube-router
    tier: node
  name: kube-router
  namespace: kube-system
spec:
  selector:
    matchLabels:
      k8s-app: kube-router
      tier: node
  template:
    metadata:
      labels:
        k8s-app: kube-router
        tier: node
      annotations:
        prometheus.io/scrape: "true"
        prometheus.io/port: "8080"
    spec:
      priorityClassName: system-node-critical
      serviceAccountName: kube-router
      initContainers:
        - name: install-cni-bins
          image: {{ .CNIInstallerImage }}
          imagePullPolicy: {{ .PullPolicy }}
          args:
            - install
          volumeMounts:
          - name: cni-bin
            mountPath: /host/opt/cni/bin
        - name: install-cniconf
          image: {{ .CNIImage }}
          imagePullPolicy: {{ .PullPolicy }}
          command:
          - /bin/sh
          - -c
          - set -e -x;
            if [ ! -f /etc/cni/net.d/10-kuberouter.conflist ]; then
              if [ -f /etc/cni/net.d/*.conf ]; then
                rm -f /etc/cni/net.d/*.conf;
              fi;
              TMP=/etc/cni/net.d/.tmp-kuberouter-cfg;
              cp /etc/kube-router/cni-conf.json ${TMP};
              mv ${TMP} /etc/cni/net.d/10-kuberouter.conflist;
            fi
          volumeMounts:
          - mountPath: /etc/cni/net.d
            name: cni-conf-dir
          - mountPath: /etc/kube-router
            name: kube-router-cfg
      hostNetwork: true
      tolerations:
      - effect: NoSchedule
        operator: Exists
      - key: CriticalAddonsOnly
        operator: Exists
      - effect: NoExecute
        operator: Exists
      volumes:
      - name: lib-modules
        hostPath:
          path: /lib/modules
      - name: cni-conf-dir
        hostPath:
          path: /etc/cni/net.d
      - name: cni-bin
        hostPath:
          path: /opt/cni/bin
          type: DirectoryOrCreate
      - name: kube-router-cfg
        configMap:
          name: kube-router-cfg
      - name: xtables-lock
        hostPath:
          path: /run/xtables.lock
          type: FileOrCreate
      containers:
      - name: kube-router
        image: {{ .CNIImage }}
        imagePullPolicy: {{ .PullPolicy }}
        args:
        - "--run-router=true"
        - "--run-firewall=true"
        - "--run-service-proxy=false"
        - "--bgp-graceful-restart=true"
        - "--metrics-port=8080"
        {{- if .PeerRouterIPs }}
        - "--peer-router-ips={{ .PeerRouterIPs }}"
        {{- end }}
        {{- if .PeerRouterASNs }}
        - "--peer-router-asns={{ .PeerRouterASNs }}"
        {{- end }}
        env:
        - name: NODE_NAME
          valueFrom:
            fieldRef:
              fieldPath: spec.nodeName
        - name: KUBE_ROUTER_CNI_CONF_FILE
          value: /etc/cni/net.d/10-kuberouter.conflist
        livenessProbe:
          httpGet:
            path: /healthz
            port: 20244
          initialDelaySeconds: 10
          periodSeconds: 3
        resources:
          requests:
            cpu: 250m
            memory: 16Mi
        securityContext:
          privileged: true
        volumeMounts:
        - name: lib-modules
          mountPath: /lib/modules
          readOnly: true
        - name: cni-conf-dir
          mountPath: /etc/cni/net.d
        - name: xtables-lock
          mountPath: /run/xtables.lock
          readOnly: false

---
apiVersion: v1
kind: ServiceAccount
metadata:
  name: kube-router
  namespace: kube-system
---
kind: ClusterRole
apiVersion: rbac.authorization.k8s.io/v1
metadata:
  name: kube-router
rules:
  - apiGroups:
    - ""
    resources:
      - namespaces
      - pods
      - services
      - nodes
      - endpoints
    verbs:
      - list
      - get
      - watch
  - apiGroups:
    - "networking.k8s.io"
    resources:
      - networkpolicies
    verbs:
      - list
      - get
      - watch
  - apiGroups:
    - extensions
    resources:
      - networkpolicies
    verbs:
      - get
      - list
      - watch

---
kind: ClusterRoleBinding
apiVersion: rbac.authorization.k8s.io/v1
metadata:
  name: kube-router
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: kube-router
subjects:
- kind: ServiceAccount
  name: kube-router
  namespace: kube-system
`
