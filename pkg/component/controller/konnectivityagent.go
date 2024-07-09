/*
Copyright 2023 k0s authors

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
	"path/filepath"
	"sync"

	"github.com/k0sproject/k0s/internal/pkg/dir"
	"github.com/k0sproject/k0s/internal/pkg/templatewriter"
	"github.com/k0sproject/k0s/pkg/apis/k0s/v1beta1"
	"github.com/k0sproject/k0s/pkg/component/manager"
	"github.com/k0sproject/k0s/pkg/component/prober"
	"github.com/k0sproject/k0s/pkg/config"
	"github.com/k0sproject/k0s/pkg/constant"
	"github.com/sirupsen/logrus"
)

type KonnectivityAgent struct {
	K0sVars                    *config.CfgVars
	APIServerHost              string
	K0sControllersLeaseCounter *K0sControllersLeaseCounter

	serverCount       int
	serverCountChan   <-chan int
	configChangeChan  chan *v1beta1.ClusterConfig
	clusterConfig     *v1beta1.ClusterConfig
	log               *logrus.Entry
	previousConfig    konnectivityAgentConfig
	agentManifestLock sync.Mutex
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
	// Subscribe to ctrl count changes
	k.serverCountChan = k.K0sControllersLeaseCounter.Subscribe()

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case count := <-k.serverCountChan:
				k.serverCount = count
				if err := k.writeKonnectivityAgent(); err != nil {
					k.log.Errorf("failed to write konnectivity agent manifest: %v", err)
				}
			case clusterConfig := <-k.configChangeChan:
				k.clusterConfig = clusterConfig
				if err := k.writeKonnectivityAgent(); err != nil {
					k.log.Errorf("failed to write konnectivity agent manifest: %v", err)
				}
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

func (k *KonnectivityAgent) writeKonnectivityAgent() error {
	k.agentManifestLock.Lock()
	defer k.agentManifestLock.Unlock()

	if k.clusterConfig == nil {
		return fmt.Errorf("cluster config is not reconciled yet")
	}

	konnectivityDir := filepath.Join(k.K0sVars.ManifestsDir, "konnectivity")
	err := dir.Init(konnectivityDir, constant.ManifestsDirMode)
	if err != nil {
		return err
	}
	cfg := konnectivityAgentConfig{
		// Since the konnectivity server runs with hostNetwork=true this is the
		// IP address of the master machine
		ProxyServerHost: k.APIServerHost,
		ProxyServerPort: uint16(k.clusterConfig.Spec.Konnectivity.AgentPort),
		Image:           k.clusterConfig.Spec.Images.Konnectivity.URI(),
		ServerCount:     k.serverCount,
		PullPolicy:      k.clusterConfig.Spec.Images.DefaultPullPolicy,
	}

	if k.clusterConfig.Spec.Network != nil {
		nllb := k.clusterConfig.Spec.Network.NodeLocalLoadBalancing
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
				return fmt.Errorf("unsupported node-local load balancer type: %q", k.clusterConfig.Spec.Network.NodeLocalLoadBalancing.Type)
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
	ProxyServerHost      string
	ProxyServerPort      uint16
	AgentPort            uint16
	Image                string
	ServerCount          int
	PullPolicy           string
	HostNetwork          bool
	BindToNodeIP         bool
	APIServerPortMapping string
	FeatureGates         string
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
              {{- if .BindToNodeIP }}
            - --bind-address=$(NODE_IP)
              {{- end }}
              {{- if .APIServerPortMapping }}
            - --apiserver-port-mapping={{ .APIServerPortMapping }}
              {{- end }}
              {{- if .FeatureGates }}
            - "--feature-gates={{ .FeatureGates }}"
              {{- end }}
          volumeMounts:
            - mountPath: /var/run/secrets/tokens
              name: konnectivity-agent-token
          livenessProbe:
            httpGet:
              port: 8093
              path: /healthz
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
