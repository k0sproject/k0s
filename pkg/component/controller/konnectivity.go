/*
Copyright 2020 k0s authors

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
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/k0sproject/k0s/internal/pkg/dir"
	"github.com/k0sproject/k0s/internal/pkg/stringmap"
	"github.com/k0sproject/k0s/internal/pkg/sysinfo/machineid"
	"github.com/k0sproject/k0s/internal/pkg/templatewriter"
	"github.com/k0sproject/k0s/internal/pkg/users"
	"github.com/k0sproject/k0s/pkg/apis/k0s/v1beta1"
	"github.com/k0sproject/k0s/pkg/assets"
	"github.com/k0sproject/k0s/pkg/component/manager"
	"github.com/k0sproject/k0s/pkg/component/prober"
	"github.com/k0sproject/k0s/pkg/config"
	"github.com/k0sproject/k0s/pkg/constant"
	kubeutil "github.com/k0sproject/k0s/pkg/kubernetes"
	"github.com/k0sproject/k0s/pkg/supervisor"
)

// Konnectivity implements the component interface of konnectivity server
type Konnectivity struct {
	K0sVars    *config.CfgVars
	LogLevel   string
	SingleNode bool
	// used for lease lock
	KubeClientFactory kubeutil.ClientFactoryInterface
	NodeConfig        *v1beta1.ClusterConfig

	supervisor          *supervisor.Supervisor
	uid                 int
	serverCount         int
	serverCountChan     chan int
	stopFunc            context.CancelFunc
	clusterConfig       *v1beta1.ClusterConfig
	log                 *logrus.Entry
	leaseCounterRunning bool
	previousConfig      konnectivityAgentConfig
	agentManifestLock   sync.Mutex
	*prober.EventEmitter
}

var _ manager.Component = (*Konnectivity)(nil)
var _ manager.Reconciler = (*Konnectivity)(nil)

// Init ...
func (k *Konnectivity) Init(_ context.Context) error {
	var err error
	k.uid, err = users.GetUID(constant.KonnectivityServerUser)
	if err != nil {
		k.EmitWithPayload("error getting UID for", err)
		logrus.Warning(fmt.Errorf("running konnectivity as root: %w", err))
	}
	err = dir.Init(k.K0sVars.KonnectivitySocketDir, 0755)
	if err != nil {
		k.EmitWithPayload("failed to initialize socket directory", err)
		return fmt.Errorf("failed to initialize directory %s: %v", k.K0sVars.KonnectivitySocketDir, err)
	}

	err = os.Chown(k.K0sVars.KonnectivitySocketDir, k.uid, -1)
	if err != nil && os.Geteuid() == 0 {
		k.EmitWithPayload("failed to chown socket directory", err)
		return fmt.Errorf("failed to chown %s: %v", k.K0sVars.KonnectivitySocketDir, err)
	}

	k.log = logrus.WithFields(logrus.Fields{"component": "konnectivity"})
	if err := assets.Stage(k.K0sVars.BinDir, "konnectivity-server", constant.BinDirMode); err != nil {
		k.EmitWithPayload("failed to stage konnectivity-server", err)
		return fmt.Errorf("failed to stage konnectivity-server binary %v", err)

	}
	defer k.Emit("succesfully initialized konnectivity component")
	return nil
}

// Run ..
func (k *Konnectivity) Start(ctx context.Context) error {
	// Buffered chan to send updates for the count of servers
	k.serverCountChan = make(chan int, 1)

	ctx, k.stopFunc = context.WithCancel(ctx)

	go k.runServer(ctx)

	return nil
}

// Reconcile detects changes in configuration and applies them to the component
func (k *Konnectivity) Reconcile(ctx context.Context, clusterCfg *v1beta1.ClusterConfig) error {
	k.clusterConfig = clusterCfg
	if !k.SingleNode {
		go k.runLeaseCounter(ctx)
	} else {
		// It's a buffered channel so once we start the runServer routine it'll pick this up and just sees it never changing
		k.serverCountChan <- 1
	}
	return k.writeKonnectivityAgent()
}

func (k *Konnectivity) defaultArgs() stringmap.StringMap {
	machineID, err := machineid.Generate()
	if err != nil {
		logrus.Errorf("failed to fetch machine ID for konnectivity-server")
	}
	return stringmap.StringMap{
		"--uds-name":                 filepath.Join(k.K0sVars.KonnectivitySocketDir, "konnectivity-server.sock"),
		"--cluster-cert":             filepath.Join(k.K0sVars.CertRootDir, "server.crt"),
		"--cluster-key":              filepath.Join(k.K0sVars.CertRootDir, "server.key"),
		"--kubeconfig":               k.K0sVars.KonnectivityKubeConfigPath,
		"--mode":                     "grpc",
		"--server-port":              "0",
		"--agent-port":               fmt.Sprintf("%d", k.clusterConfig.Spec.Konnectivity.AgentPort),
		"--admin-port":               fmt.Sprintf("%d", k.clusterConfig.Spec.Konnectivity.AdminPort),
		"--agent-namespace":          "kube-system",
		"--agent-service-account":    "konnectivity-agent",
		"--authentication-audience":  "system:konnectivity-server",
		"--logtostderr":              "true",
		"--stderrthreshold":          "1",
		"--v":                        k.LogLevel,
		"--enable-profiling":         "false",
		"--delete-existing-uds-file": "true",
		"--server-id":                machineID.ID(),
		"--proxy-strategies":         "destHost,default",
		"--cipher-suites":            constant.AllowedTLS12CipherSuiteNames(),
	}
}

// runs the supervisor and restarts if the calculated server count changes
func (k *Konnectivity) runServer(ctx context.Context) {
	previousArgs := stringmap.StringMap{}
	for {
		select {
		case <-ctx.Done():
			k.Emit("stopped konnectivity server")
			logrus.Info("stopping konnectivity server reconfig loop")
			return
		case count := <-k.serverCountChan:
			if k.clusterConfig == nil {
				k.Emit("skipping konnectivity server start, cluster config not yet available")
				continue
			}
			// restart only if the count actually changes and we've got the global config
			if count != k.serverCount && k.clusterConfig != nil {
				args := k.defaultArgs()
				args["--server-count"] = strconv.Itoa(count)
				if args.Equals(previousArgs) {
					logrus.Info("no changes detected for konnectivity-server")
				}
				// Stop supervisor
				if k.supervisor != nil {
					k.EmitWithPayload("restarting konnectivity server due to server count change",
						map[string]interface{}{"serverCount": count})
					if err := k.supervisor.Stop(); err != nil {
						logrus.Errorf("failed to stop supervisor: %s", err)
						// TODO Should we just return? That means other part will continue to run but the server is never properly restarted
					}
				}

				k.supervisor = &supervisor.Supervisor{
					Name:    "konnectivity",
					BinPath: assets.BinPath("konnectivity-server", k.K0sVars.BinDir),
					DataDir: k.K0sVars.DataDir,
					RunDir:  k.K0sVars.RunDir,
					Args:    args.ToArgs(),
					UID:     k.uid,
				}
				err := k.supervisor.Supervise()
				if err != nil {
					k.EmitWithPayload("failed to run konnectivity server", err)
					logrus.Errorf("failed to start konnectivity supervisor: %s", err)
					k.supervisor = nil // not to make the next loop to try to stop it first
					continue
				}
				k.serverCount = count

				if err := k.writeKonnectivityAgent(); err != nil {
					k.EmitWithPayload("failed to write konnectivity agent config", err)
					logrus.Errorf("failed to update konnectivity-agent template: %s", err)
				}
				k.EmitWithPayload("started konnectivity server", map[string]interface{}{"serverCount": count})
			}
		}
	}
}

// Stop stops
func (k *Konnectivity) Stop() error {
	if k.stopFunc != nil {
		logrus.Debug("closing konnectivity component context")
		k.stopFunc()
	}
	if k.supervisor == nil {
		return nil
	}
	logrus.Debug("about to stop konnectivity supervisor")
	return k.supervisor.Stop()
}

func (k *Konnectivity) Healthy() error {
	if k.clusterConfig == nil {
		return fmt.Errorf("cluster config not yet available")
	}

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

func (k *Konnectivity) writeKonnectivityAgent() error {
	k.agentManifestLock.Lock()
	defer k.agentManifestLock.Unlock()
	konnectivityDir := filepath.Join(k.K0sVars.ManifestsDir, "konnectivity")
	err := dir.Init(konnectivityDir, constant.ManifestsDirMode)
	if err != nil {
		return err
	}
	cfg := konnectivityAgentConfig{
		// Since the konnectivity server runs with hostNetwork=true this is the
		// IP address of the master machine
		ProxyServerHost: k.NodeConfig.Spec.API.APIAddress(), // TODO: should it be an APIAddress?
		ProxyServerPort: uint16(k.clusterConfig.Spec.Konnectivity.AgentPort),
		Image:           k.clusterConfig.Spec.Images.Konnectivity.URI(),
		ServerCount:     k.serverCount,
		PullPolicy:      k.clusterConfig.Spec.Images.DefaultPullPolicy,
	}

	if k.NodeConfig.Spec.API.TunneledNetworkingMode {
		cfg.HostNetwork = true
		cfg.BindToNodeIP = true // agent needs to listen on the node IP to be on pair with the tunneled network reconciler
		cfg.APIServerPortMapping = fmt.Sprintf("6443:localhost:%d", k.clusterConfig.Spec.API.Port)
	} else {
		cfg.FeatureGates = "NodeToMasterTraffic=false"
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
		return fmt.Errorf("failed to write konnectivity agent manifest: %v", err)
	}
	k.previousConfig = cfg
	k.Emit("wrote konnectivity agent new manifest")
	return nil
}

func (k *Konnectivity) runLeaseCounter(ctx context.Context) {
	if k.leaseCounterRunning {
		return
	}
	k.leaseCounterRunning = true
	logrus.Infof("starting to count controller lease holders every 10 secs")
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			logrus.Info("stopping konnectivity lease counter")
			return
		case <-ticker.C:
			count, err := k.countLeaseHolders(ctx)
			if err != nil {
				logrus.Errorf("failed to count controller leases: %s", err)
				continue
			}
			k.serverCountChan <- count
		}
	}
}

func (k *Konnectivity) countLeaseHolders(ctx context.Context) (int, error) {
	client, err := k.KubeClientFactory.GetClient()
	if err != nil {
		return 0, err
	}

	return kubeutil.GetControlPlaneNodeCount(ctx, client)
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
