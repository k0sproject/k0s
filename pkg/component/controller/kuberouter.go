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
	"context"
	"fmt"
	"reflect"

	"github.com/k0sproject/k0s/internal/pkg/stringmap"
	"github.com/k0sproject/k0s/internal/pkg/templatewriter"
	"github.com/k0sproject/k0s/pkg/apis/k0s/v1beta1"
	"github.com/k0sproject/k0s/pkg/component/manager"
	"github.com/k0sproject/k0s/pkg/config"
	"github.com/k0sproject/k0s/pkg/constant"

	"github.com/sirupsen/logrus"
)

// KubeRouter implements the kube-router reconciler component
type KubeRouter struct {
	log logrus.FieldLogger

	saver   manifestsSaver
	k0sVars *config.CfgVars

	previousConfig kubeRouterConfig
}

var _ manager.Component = (*KubeRouter)(nil)
var _ manager.Reconciler = (*KubeRouter)(nil)

type kubeRouterConfig struct {
	MTU               int
	AutoMTU           bool
	MetricsPort       int
	CNIInstallerImage string
	CNIImage          string
	CNIHairpin        bool
	IPMasq            bool
	PeerRouterIPs     string
	PeerRouterASNs    string
	PullPolicy        string
	Args              []string
}

// NewKubeRouter creates new KubeRouter reconciler component
func NewKubeRouter(k0sVars *config.CfgVars, manifestsSaver manifestsSaver) *KubeRouter {
	return &KubeRouter{
		log: logrus.WithFields(logrus.Fields{"component": "kube-router"}),

		saver:   manifestsSaver,
		k0sVars: k0sVars,
	}
}

// Init does nothing
func (k *KubeRouter) Init(_ context.Context) error { return nil }

// Stop no-op as nothing running
func (k *KubeRouter) Stop() error { return nil }

func getHairpinConfig(krc *v1beta1.KubeRouter) (cniHairpin bool, globalHairpin bool) {
	// Configure hairpin
	switch krc.Hairpin {
	case v1beta1.HairpinUndefined:
		// If Hairpin is undefined, then we honor HairpinMode
		if krc.HairpinMode {
			cniHairpin = true
			globalHairpin = true
		}
	case v1beta1.HairpinDisabled:
		cniHairpin = false
		globalHairpin = false
	case v1beta1.HairpinAllowed:
		cniHairpin = true
		globalHairpin = false
	case v1beta1.HairpinEnabled:
		cniHairpin = true
		globalHairpin = true
	}
	return
}

// Reconcile detects changes in configuration and applies them to the component
func (k *KubeRouter) Reconcile(_ context.Context, clusterConfig *v1beta1.ClusterConfig) error {
	logrus.Debug("reconcile method called for: KubeRouter")
	if clusterConfig.Spec.Network.Provider != constant.CNIProviderKubeRouter {
		return nil
	}

	existingCNI := existingCNIProvider(k.k0sVars.ManifestsDir)
	if existingCNI != "" && existingCNI != constant.CNIProviderKubeRouter {
		return fmt.Errorf("cannot change CNI provider from %s to %s", existingCNI, constant.CNIProviderKubeRouter)
	}

	cniHairpin, globalHairpin := getHairpinConfig(clusterConfig.Spec.Network.KubeRouter)

	args := stringmap.StringMap{
		// k0s set default args
		"run-router":           "true",
		"run-firewall":         "true",
		"run-service-proxy":    "false",
		"bgp-graceful-restart": "true",
		"enable-ipv4":          "true",
		// Args from config values
		"enable-ipv6":  fmt.Sprintf("%t", clusterConfig.Spec.Network.DualStack.Enabled),
		"auto-mtu":     fmt.Sprintf("%t", clusterConfig.Spec.Network.KubeRouter.IsAutoMTU()),
		"metrics-port": fmt.Sprintf("%d", clusterConfig.Spec.Network.KubeRouter.MetricsPort),
		"hairpin-mode": fmt.Sprintf("%t", globalHairpin),
	}

	// We should not add peering flags if the values are empty
	if clusterConfig.Spec.Network.KubeRouter.PeerRouterASNs != "" {
		args["peer-router-asns"] = clusterConfig.Spec.Network.KubeRouter.PeerRouterASNs
	}
	if clusterConfig.Spec.Network.KubeRouter.PeerRouterIPs != "" {
		args["peer-router-ips"] = clusterConfig.Spec.Network.KubeRouter.PeerRouterIPs
	}

	// Override or add args from config
	args.Merge(clusterConfig.Spec.Network.KubeRouter.ExtraArgs)

	cfg := kubeRouterConfig{
		AutoMTU:           clusterConfig.Spec.Network.KubeRouter.IsAutoMTU(),
		MTU:               clusterConfig.Spec.Network.KubeRouter.MTU,
		MetricsPort:       clusterConfig.Spec.Network.KubeRouter.MetricsPort,
		IPMasq:            clusterConfig.Spec.Network.KubeRouter.IPMasq,
		CNIHairpin:        cniHairpin,
		CNIImage:          clusterConfig.Spec.Images.KubeRouter.CNI.URI(),
		CNIInstallerImage: clusterConfig.Spec.Images.KubeRouter.CNIInstaller.URI(),
		PullPolicy:        clusterConfig.Spec.Images.DefaultPullPolicy,
		Args:              args.ToDashedArgs(),
	}

	if reflect.DeepEqual(k.previousConfig, cfg) {
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
		return fmt.Errorf("error writing kube-router manifests, will NOT retry: %w", err)
	}

	err = k.saver.Save("kube-router.yaml", output.Bytes())
	if err != nil {
		return fmt.Errorf("error writing kube-router manifests, will NOT retry: %w", err)
	}
	return nil
}

// Run runs the kube-router reconciler
func (k *KubeRouter) Start(_ context.Context) error {
	k.log.Info("starting to dump manifests")

	return nil
}

// from https://github.com/cloudnativelabs/kube-router/blob/master/daemonset/
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
             "bridge":"kube-bridge",
             "isDefaultGateway":true,
             "hairpinMode": {{ .CNIHairpin }},
             "ipMasq": {{ .IPMasq }},
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
      {{- if gt .MetricsPort 0 }}
      annotations:
        prometheus.io/scrape: "true"
        prometheus.io/port: "{{ .MetricsPort }}"
      {{- end }}
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
      hostPID: true
      tolerations:
      - effect: NoSchedule
        operator: Exists
      - key: CriticalAddonsOnly
        operator: Exists
      - effect: NoExecute
        operator: Exists
      - key: "node-role.kubernetes.io/master"
        operator: "Exists"
        effect: "NoSchedule"
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
        {{- range .Args }}
        - {{ . | printf "%q" }}
        {{- end }}
        env:
        - name: NODE_NAME
          valueFrom:
            fieldRef:
              fieldPath: spec.nodeName
        - name: POD_NAME
          valueFrom:
            fieldRef:
              fieldPath: metadata.name
        - name: KUBE_ROUTER_CNI_CONF_FILE
          value: /etc/cni/net.d/10-kuberouter.conflist
        ports:
        - name: healthz
          containerPort: 20244
        livenessProbe:
          httpGet:
            path: /healthz
            port: healthz
          initialDelaySeconds: 300
          periodSeconds: 10
          timeoutSeconds: 10
          failureThreshold: 6
        readinessProbe:
          httpGet:
            path: /healthz
            port: healthz
          periodSeconds: 3
          timeoutSeconds: 3
          failureThreshold: 3
          successThreshold: 3
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
  - apiGroups:
      - "coordination.k8s.io"
    resources:
      - leases
    verbs:
      - get
      - create
      - update
  - apiGroups:
      - ""
    resources:
      - services/status
    verbs:
      - update
  - apiGroups:
      - "discovery.k8s.io"
    resources:
      - endpointslices
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
