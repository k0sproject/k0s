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
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/k0sproject/k0s/internal/pkg/dir"
	"github.com/k0sproject/k0s/internal/pkg/machineid"
	"github.com/k0sproject/k0s/internal/pkg/stringmap"
	"github.com/k0sproject/k0s/internal/pkg/templatewriter"
	"github.com/k0sproject/k0s/internal/pkg/users"
	"github.com/k0sproject/k0s/pkg/apis/k0s.k0sproject.io/v1beta1"
	"github.com/k0sproject/k0s/pkg/assets"
	"github.com/k0sproject/k0s/pkg/component"
	"github.com/k0sproject/k0s/pkg/constant"
	k8sutil "github.com/k0sproject/k0s/pkg/kubernetes"
	kubeutil "github.com/k0sproject/k0s/pkg/kubernetes"
	"github.com/k0sproject/k0s/pkg/supervisor"
)

// Konnectivity implements the component interface of konnectivity server
type Konnectivity struct {
	K0sVars  constant.CfgVars
	LogLevel string
	// used for lease lock
	KubeClientFactory k8sutil.ClientFactoryInterface
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
}

var _ component.Component = &Konnectivity{}
var _ component.ReconcilerComponent = &Konnectivity{}

// Init ...
func (k *Konnectivity) Init() error {
	var err error
	k.uid, err = users.GetUID(constant.KonnectivityServerUser)
	if err != nil {
		logrus.Warning(fmt.Errorf("running konnectivity as root: %w", err))
	}
	err = dir.Init(k.K0sVars.KonnectivitySocketDir, 0755)
	if err != nil {
		return fmt.Errorf("failed to initialize directory %s: %v", k.K0sVars.KonnectivitySocketDir, err)
	}

	err = os.Chown(k.K0sVars.KonnectivitySocketDir, k.uid, -1)
	if err != nil && os.Geteuid() == 0 {
		return fmt.Errorf("failed to chown %s: %v", k.K0sVars.KonnectivitySocketDir, err)
	}

	k.log = logrus.WithFields(logrus.Fields{"component": "konnectivity"})

	return assets.Stage(k.K0sVars.BinDir, "konnectivity-server", constant.BinDirMode)
}

// Run ..
func (k *Konnectivity) Run(ctx context.Context) error {
	// Buffered chan to send updates for the count of servers
	k.serverCountChan = make(chan int, 1)

	ctx, k.stopFunc = context.WithCancel(ctx)

	go k.runServer(ctx)

	return nil
}

// Reconcile detects changes in configuration and applies them to the component
func (k *Konnectivity) Reconcile(ctx context.Context, clusterCfg *v1beta1.ClusterConfig) error {
	k.clusterConfig = clusterCfg
	if k.NodeConfig.Spec.API.ExternalAddress != "" {
		go k.runLeaseCounter(ctx)
	} else {
		// It's a buffered channel so once we start the runServer routine it'll pick this up and just sees it never changing
		k.serverCountChan <- 1
	}
	return k.writeKonnectivityAgent()
}

func (k *Konnectivity) defaultArgs() stringmap.StringMap {
	serverID, err := machineid.Generate()
	if err != nil {
		logrus.Errorf("failed to fetch server ID for konnectivity-server")
	}
	return stringmap.StringMap{
		"--uds-name":                filepath.Join(k.K0sVars.KonnectivitySocketDir, "konnectivity-server.sock"),
		"--cluster-cert":            filepath.Join(k.K0sVars.CertRootDir, "server.crt"),
		"--cluster-key":             filepath.Join(k.K0sVars.CertRootDir, "server.key"),
		"--kubeconfig":              k.K0sVars.KonnectivityKubeConfigPath,
		"--mode":                    "grpc",
		"--server-port":             "0",
		"--agent-port":              fmt.Sprintf("%d", k.clusterConfig.Spec.Konnectivity.AgentPort),
		"--admin-port":              fmt.Sprintf("%d", k.clusterConfig.Spec.Konnectivity.AdminPort),
		"--agent-namespace":         "kube-system",
		"--agent-service-account":   "konnectivity-agent",
		"--authentication-audience": "system:konnectivity-server",
		"--logtostderr":             "true",
		"--stderrthreshold":         "1",
		"--v":                       k.LogLevel,
		"--enable-profiling":        "false",
		"--server-id":               serverID,
	}
}

// runs the supervisor and restarts if the calculated server count changes
func (k *Konnectivity) runServer(ctx context.Context) {
	previousArgs := stringmap.StringMap{}
	for {
		select {
		case <-ctx.Done():
			logrus.Info("stopping konnectivity server reconfig loop")
			return
		case count := <-k.serverCountChan:
			// restart only if the count actually changes and we've got the global config
			if count != k.serverCount && k.clusterConfig != nil {
				args := k.defaultArgs()
				args["--server-count"] = strconv.Itoa(count)
				if args.Equals(previousArgs) {
					logrus.Info("no changes detected for konnectivity-server")
				}
				// Stop supervisor
				if k.supervisor != nil {
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
					logrus.Errorf("failed to start konnectivity supervisor: %s", err)
					k.supervisor = nil // not to make the next loop to try to stop it first
					continue
				}
				k.serverCount = count

				if err := k.writeKonnectivityAgent(); err != nil {
					logrus.Errorf("failed to update konnectivity-agent template: %s", err)
				}
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

type konnectivityAgentConfig struct {
	APIAddress  string
	AgentPort   int64
	Image       string
	ServerCount int
	PullPolicy  string
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
		APIAddress:  k.NodeConfig.Spec.API.APIAddress(),
		AgentPort:   k.clusterConfig.Spec.Konnectivity.AgentPort,
		Image:       k.clusterConfig.Spec.Images.Konnectivity.URI(),
		ServerCount: k.serverCount,
		PullPolicy:  k.clusterConfig.Spec.Images.DefaultPullPolicy,
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
		return fmt.Errorf("failed to write konnectivity agent manifest: %v", err)
	}
	k.previousConfig = cfg
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
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	count := 0
	leases, err := client.CoordinationV1().Leases("kube-node-lease").List(ctx, v1.ListOptions{})
	if err != nil {
		return 0, err
	}
	for _, l := range leases.Items {
		if strings.HasPrefix(l.ObjectMeta.Name, "k0s-ctrl") {
			if kubeutil.IsValidLease(l) {
				count++
			}
		}
	}

	return count, nil
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
    spec:
      nodeSelector:
        kubernetes.io/os: linux
      priorityClassName: system-cluster-critical
      tolerations:
        - key: "CriticalAddonsOnly"
          operator: "Exists"
        - key: "node-role.kubernetes.io/master"
          operator: "Exists"
          effect: "NoSchedule"
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
          args: [
                  "--logtostderr=true",
                  "--ca-cert=/var/run/secrets/kubernetes.io/serviceaccount/ca.crt",
                  # Since the konnectivity server runs with hostNetwork=true,
                  # this is the IP address of the master machine.
                  "--proxy-server-host={{ .APIAddress }}",
                  "--proxy-server-port={{ .AgentPort }}",
                  "--service-account-token-path=/var/run/secrets/tokens/konnectivity-agent-token"
                  ]
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

// Healthy is a no-op check
func (k *Konnectivity) Healthy() error { return nil }
