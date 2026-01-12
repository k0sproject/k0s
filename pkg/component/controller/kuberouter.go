// SPDX-FileCopyrightText: 2021 k0s authors
// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"bytes"
	"context"
	"fmt"
	"path/filepath"
	"reflect"
	"strconv"

	"github.com/k0sproject/k0s/internal/pkg/dir"
	"github.com/k0sproject/k0s/internal/pkg/file"
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
func NewKubeRouter(k0sVars *config.CfgVars) *KubeRouter {
	return &KubeRouter{
		log: logrus.WithFields(logrus.Fields{"component": "kube-router"}),

		k0sVars: k0sVars,
	}
}

// Init implements [manager.Component].
func (k *KubeRouter) Init(context.Context) error {
	return dir.Init(filepath.Join(k.k0sVars.ManifestsDir, "kuberouter"), constant.ManifestsDirMode)
}

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

	isSingleStackIPv6 := clusterConfig.Spec.Network.IsSingleStackIPv6()
	args := stringmap.StringMap{
		// k0s set default args
		"run-router":           "true",
		"run-firewall":         "true",
		"run-service-proxy":    "false",
		"bgp-graceful-restart": "true",
		// Args from config values
		"enable-ipv4":              strconv.FormatBool(!isSingleStackIPv6),
		"enable-ipv6":              strconv.FormatBool(clusterConfig.Spec.Network.DualStack.Enabled || isSingleStackIPv6),
		"auto-mtu":                 strconv.FormatBool(clusterConfig.Spec.Network.KubeRouter.IsAutoMTU()),
		"metrics-port":             strconv.Itoa(clusterConfig.Spec.Network.KubeRouter.MetricsPort),
		"hairpin-mode":             strconv.FormatBool(globalHairpin),
		"service-cluster-ip-range": clusterConfig.Spec.Network.ServiceCIDR,
	}

	// IPv6 requires a router ID, instead of generating one ourselves, rely on kube-router logic
	if isSingleStackIPv6 {
		args["router-id"] = "generate"
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

	// Always set --master flag to the effective API server URL (considering NLLB)
	args["master"] = clusterConfig.Spec.APIServerURLForHostNetworkPods()

	// Warn if kube-proxy is disabled but no service proxy is configured
	if clusterConfig.Spec.Network.KubeProxy.Disabled && args["run-service-proxy"] != "true" {
		k.log.Warn("kube-proxy is disabled but kube-router is not configured to run service proxy (run-service-proxy: true). Ensure another component is providing service proxy functionality.")
	}

	cfg := kubeRouterConfig{
		AutoMTU:           clusterConfig.Spec.Network.KubeRouter.IsAutoMTU(),
		MTU:               clusterConfig.Spec.Network.KubeRouter.MTU,
		MetricsPort:       clusterConfig.Spec.Network.KubeRouter.MetricsPort,
		IPMasq:            clusterConfig.Spec.Network.KubeRouter.IPMasq,
		CNIHairpin:        cniHairpin,
		CNIImage:          clusterConfig.Spec.Images.KubeRouter.CNI.URI(),
		CNIInstallerImage: clusterConfig.Spec.Images.KubeRouter.CNIInstaller.URI(),
		PullPolicy:        clusterConfig.Spec.Images.DefaultPullPolicy,
		Args:              append(args.ToDashedArgs(), clusterConfig.Spec.Network.KubeRouter.RawArgs...),
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

	if err := file.AtomicWithTarget(filepath.Join(k.k0sVars.ManifestsDir, "kuberouter", "kube-router.yaml")).
		WithPermissions(constant.CertMode).
		Write(output.Bytes()); err != nil {
		return fmt.Errorf("error writing kube-router manifests, will NOT retry: %w", err)
	}

	return nil
}

// Start implements [manager.Component].
func (k *KubeRouter) Start(context.Context) error {
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
      nodeSelector:
        kubernetes.io/os: linux
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
