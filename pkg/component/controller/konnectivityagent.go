// SPDX-FileCopyrightText: 2023 k0s authors
// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"context"
	"fmt"
	"path/filepath"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/k0sproject/k0s/internal/pkg/dir"
	k0snet "github.com/k0sproject/k0s/internal/pkg/net"
	"github.com/k0sproject/k0s/internal/pkg/templatewriter"
	"github.com/k0sproject/k0s/pkg/apis/k0s/v1beta1"
	"github.com/k0sproject/k0s/pkg/component/manager"
	"github.com/k0sproject/k0s/pkg/component/prober"
	"github.com/k0sproject/k0s/pkg/constant"
)

type KonnectivityAgent struct {
	ManifestsDir           string
	KonnectivityServerHost string
	ServerCount            func() (uint, <-chan struct{})

	configChangeChan chan *v1beta1.ClusterConfig
	log              *logrus.Entry
	previousConfig   konnectivityAgentConfig
	*prober.EventEmitter
}

var _ manager.Component = (*KonnectivityAgent)(nil)
var _ manager.Reconciler = (*KonnectivityAgent)(nil)

func (k *KonnectivityAgent) Init(_ context.Context) error {
	k.log = logrus.WithFields(logrus.Fields{"component": "konnectivity-agent"})

	k.configChangeChan = make(chan *v1beta1.ClusterConfig, 1)

	return nil
}

func (k *KonnectivityAgent) Start(ctx context.Context) error {

	go func() {
		serverCount, serverCountChanged := k.ServerCount()

		var clusterConfig *v1beta1.ClusterConfig
		var retry <-chan time.Time
		for {
			select {
			case config := <-k.configChangeChan:
				clusterConfig = config

			case <-serverCountChanged:
				prevServerCount := serverCount
				serverCount, serverCountChanged = k.ServerCount()
				// write only if the server count actually changed
				if serverCount == prevServerCount {
					continue
				}

			case <-retry:
				k.log.Info("Retrying to write konnectivity agent manifest")

			case <-ctx.Done():
				return
			}

			retry = nil

			if clusterConfig == nil {
				k.log.Info("Cluster configuration has not yet been reconciled")
				continue
			}

			if err := k.writeKonnectivityAgent(clusterConfig, serverCount); err != nil {
				k.log.Errorf("failed to write konnectivity agent manifest: %v", err)
				retry = time.After(10 * time.Second)
				continue
			}
		}
	}()

	return nil
}

// Reconcile detects changes in configuration and applies them to the component
func (k *KonnectivityAgent) Reconcile(ctx context.Context, clusterCfg *v1beta1.ClusterConfig) error {
	k.configChangeChan <- clusterCfg

	return nil
}

func (k *KonnectivityAgent) Stop() error {
	return nil
}

func (k *KonnectivityAgent) writeKonnectivityAgent(clusterConfig *v1beta1.ClusterConfig, serverCount uint) error {
	konnectivityDir := filepath.Join(k.ManifestsDir, "konnectivity")
	err := dir.Init(konnectivityDir, constant.ManifestsDirMode)
	if err != nil {
		return err
	}

	cfg := konnectivityAgentConfig{
		ProxyServerHost: k.KonnectivityServerHost,
		ProxyServerPort: uint16(clusterConfig.Spec.Konnectivity.AgentPort),
		Image:           clusterConfig.Spec.Images.Konnectivity.URI(),
		ServerCount:     serverCount,
		PullPolicy:      clusterConfig.Spec.Images.DefaultPullPolicy,
		HostNetwork:     clusterConfig.Spec.Konnectivity.HostNetwork,
	}

	if externalAddress := clusterConfig.Spec.Konnectivity.ExternalAddress; externalAddress != "" {
		serverHostPort, err := k0snet.ParseHostPortWithDefault(externalAddress, cfg.ProxyServerPort)
		if err != nil {
			return fmt.Errorf("failed to determine proxy server host and port (%q, %d): %w", externalAddress, cfg.ProxyServerPort, err)
		}
		cfg.ProxyServerHost, cfg.ProxyServerPort = serverHostPort.Host(), serverHostPort.Port()
	} else if clusterConfig.Spec.Network != nil {
		nllb := clusterConfig.Spec.Network.NodeLocalLoadBalancing
		if nllb.IsEnabled() {
			switch nllb.Type {
			case v1beta1.NllbTypeEnvoyProxy:
				k.log.Debugf("Enabling node-local load balancing via %s", nllb.Type)

				// FIXME: Transitions from non-node-local load balanced to
				// node-local load balanced setups will be problematic: The
				// controller will update the DaemonSet with localhost, but the
				// worker nodes won't reconcile their state (yet) and need to be
				// restarted manually in order to start their load balancer.
				// Transitions in the other direction suffer from the same
				// limitation, but that will be less grave, as the node-local
				// load balancers will remain operational until the next node
				// restart and the agents will stay connected.

				// The node-local load balancer will run in the host network, so
				// the agent needs to do the same in order to use it.
				cfg.HostNetwork = true

				// FIXME: This is not exactly on par with the way it's
				// implemented on the worker side, i.e. there's no fallback if
				// localhost doesn't resolve to a loopback address. But this
				// would require some shenanigans to pull in node-specific
				// values here. A possible solution would be to convert the
				// konnectivity agent to a static Pod as well.
				cfg.ProxyServerHost = "localhost"

				if nllb.EnvoyProxy.KonnectivityServerBindPort != nil {
					cfg.ProxyServerPort = uint16(*nllb.EnvoyProxy.KonnectivityServerBindPort)
				} else {
					cfg.ProxyServerPort = uint16(*v1beta1.DefaultEnvoyProxy().KonnectivityServerBindPort)
				}
			default:
				return fmt.Errorf("unsupported node-local load balancer type: %q", clusterConfig.Spec.Network.NodeLocalLoadBalancing.Type)
			}
		}
	}

	if cfg == k.previousConfig {
		k.log.Debug("agent configs match, no need to reconcile")
		return nil
	}

	tw := templatewriter.TemplateWriter{
		Name:     "konnectivity-agent",
		Template: konnectivityAgentTemplate,
		Data:     cfg,
		Path:     filepath.Join(konnectivityDir, "konnectivity-agent.yaml"),
	}
	err = tw.Write()
	if err != nil {
		k.EmitWithPayload("failed to write konnectivity agent manifest", err)
		return fmt.Errorf("failed to write konnectivity agent manifest: %w", err)
	}
	k.previousConfig = cfg
	k.Emit("wrote konnectivity agent new manifest")
	return nil
}

type konnectivityAgentConfig struct {
	ProxyServerHost string
	ProxyServerPort uint16
	Image           string
	ServerCount     uint
	PullPolicy      string
	HostNetwork     bool
}

const konnectivityAgentTemplate = `
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: system:konnectivity-server
  labels:
    kubernetes.io/cluster-service: "true"
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: system:auth-delegator
subjects:
  - apiGroup: rbac.authorization.k8s.io
    kind: User
    name: system:konnectivity-server
---
apiVersion: v1
kind: ServiceAccount
metadata:
  name: konnectivity-agent
  namespace: kube-system
  labels:
    kubernetes.io/cluster-service: "true"
---
apiVersion: apps/v1
# Alternatively, you can deploy the agents as Deployments. It is not necessary
# to have an agent on each node.
kind: DaemonSet
metadata:
  labels:
    k8s-app: konnectivity-agent
  namespace: kube-system
  name: konnectivity-agent
spec:
  selector:
    matchLabels:
      k8s-app: konnectivity-agent
  template:
    metadata:
      labels:
        k8s-app: konnectivity-agent
      annotations:
        prometheus.io/scrape: 'true'
        prometheus.io/port: '8093'
    spec:
      securityContext:
        allowPrivilegeEscalation: false
        capabilities:
          drop:
          - all
        readOnlyRootFilesystem: true
        runAsNonRoot: true
        supplementalGroups: [0]` /* in order to read the projected service account token */ + `
      nodeSelector:
        kubernetes.io/os: linux
      priorityClassName: system-cluster-critical
      tolerations:
        - operator: Exists
      {{- if .HostNetwork }}
      hostNetwork: true
      {{- end }}
      containers:
        - image: {{ .Image }}
          imagePullPolicy: {{ .PullPolicy }}
          name: konnectivity-agent
          command: ["/proxy-agent"]
          env:
              # the variable is not in a use
              # we need it to have agent restarted on server count change
              - name: K0S_CONTROLLER_COUNT
                value: "{{ .ServerCount }}"

              - name: NODE_IP
                valueFrom:
                  fieldRef:
                    fieldPath: status.hostIP
          args:
            - --logtostderr=true
            - --ca-cert=/var/run/secrets/kubernetes.io/serviceaccount/ca.crt
            - --proxy-server-host={{ .ProxyServerHost }}
            - --proxy-server-port={{ .ProxyServerPort }}
            - --service-account-token-path=/var/run/secrets/tokens/konnectivity-agent-token
            - --agent-identifiers=host=$(NODE_IP)
            - --agent-id=$(NODE_IP)
          volumeMounts:
            - mountPath: /var/run/secrets/tokens
              name: konnectivity-agent-token
          livenessProbe:
            httpGet:
              port: 8093
              path: /healthz
            initialDelaySeconds: 15
            timeoutSeconds: 15
          readinessProbe:` /* helps to quickly identify pods with connection issues */ + `
            httpGet:
              port: 8093
              path: /readyz
            initialDelaySeconds: 15
            timeoutSeconds: 15
      serviceAccountName: konnectivity-agent
      volumes:
        - name: konnectivity-agent-token
          projected:
            sources:
              - serviceAccountToken:
                  path: konnectivity-agent-token
                  audience: system:konnectivity-server
`
