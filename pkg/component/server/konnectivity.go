/*
Copyright 2020 Mirantis, Inc.

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
package server

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/k0sproject/k0s/internal/util"
	config "github.com/k0sproject/k0s/pkg/apis/v1beta1"
	"github.com/k0sproject/k0s/pkg/assets"
	"github.com/k0sproject/k0s/pkg/constant"
	"github.com/k0sproject/k0s/pkg/kubernetes"
	k8sutil "github.com/k0sproject/k0s/pkg/kubernetes"
	"github.com/k0sproject/k0s/pkg/supervisor"
)

const (
	konnectivityLeaseName      = "konnectivity-server"
	konnectivityLeaseNameSpace = "kube-node-lease"
	serviceName                = "konnectivity-server"
)

// Konnectivity implement the component interface of konnectivity server
type Konnectivity struct {
	ClusterConfig *config.ClusterConfig
	K0sVars       constant.CfgVars
	LogLevel      string
	supervisor    supervisor.Supervisor
	uid           int

	// used for lease lock
	KubeClientFactory k8sutil.ClientFactory

	serverCount int
	leaseLock   *kubernetes.LeaseLock
	done        chan bool
}

// Init ...
func (k *Konnectivity) Init() error {
	var err error
	k.uid, err = util.GetUID(constant.KonnectivityServerUser)
	if err != nil {
		logrus.Warning(fmt.Errorf("Running konnectivity as root: %v", err))
	}
	err = util.InitDirectory(k.K0sVars.KonnectivitySocketDir, 0755)
	if err != nil {
		return fmt.Errorf("failed to initialize directory %s: %v", k.K0sVars.KonnectivitySocketDir, err)
	}

	err = os.Chown(k.K0sVars.KonnectivitySocketDir, k.uid, -1)
	if err != nil && os.Geteuid() == 0 {
		return fmt.Errorf("failed to chown %s: %v", k.K0sVars.KonnectivitySocketDir, err)
	}

	// set default serverCount to 1
	k.serverCount = kubernetes.MinLeaseHolders
	return assets.Stage(k.K0sVars.BinDir, "konnectivity-server", constant.BinDirMode)
}

// Run ..
func (k *Konnectivity) Run() error {
	args := []string{
		fmt.Sprintf("--uds-name=%s", filepath.Join(k.K0sVars.KonnectivitySocketDir, "konnectivity-server.sock")),
		fmt.Sprintf("--cluster-cert=%s", filepath.Join(k.K0sVars.CertRootDir, "server.crt")),
		fmt.Sprintf("--cluster-key=%s", filepath.Join(k.K0sVars.CertRootDir, "server.key")),
		fmt.Sprintf("--kubeconfig=%s", k.K0sVars.KonnectivityKubeConfigPath),
		"--mode=grpc",
		"--server-port=0",
		"--agent-port=8132",
		"--admin-port=8133",
		"--agent-namespace=kube-system",
		"--agent-service-account=konnectivity-agent",
		"--authentication-audience=system:konnectivity-server",
		"--logtostderr=true",
		"--stderrthreshold=1",
		"-v=2",
		fmt.Sprintf("--v=%s", k.LogLevel),
		"--enable-profiling=false",
	}

	if k.ClusterConfig.Spec.API.ExternalAddress != "" {
		serverID, err := util.MachineID()
		if err != nil {
			logrus.Errorf("failed to fetch server ID for %v", serviceName)
		}
		args = append(args, fmt.Sprintf("--server-count=%d", k.serverCount))
		args = append(args, fmt.Sprintf("--server-id=%s", serverID))
	}

	logrus.Info("Starting konnectivity")
	k.supervisor = supervisor.Supervisor{
		Name:    "konnectivity",
		BinPath: assets.BinPath("konnectivity-server", k.K0sVars.BinDir),
		DataDir: k.K0sVars.DataDir,
		RunDir:  k.K0sVars.RunDir,
		Args:    args,
		UID:     k.uid,
	}

	err := k.supervisor.Supervise()
	if err != nil {
		return err
	}
	if k.ClusterConfig.Spec.API.ExternalAddress != "" {
		k.runLease()
	}
	return k.writeKonnectivityAgent()
}

// Stop stops
func (k *Konnectivity) Stop() error {
	if k.ClusterConfig.Spec.API.ExternalAddress != "" {
		k.stopLease()
	}
	return k.supervisor.Stop()
}

type konnectivityAgentConfig struct {
	APIAddress string
	Image      string
}

func (k *Konnectivity) writeKonnectivityAgent() error {
	konnectivityDir := filepath.Join(k.K0sVars.ManifestsDir, "konnectivity")
	err := util.InitDirectory(konnectivityDir, constant.ManifestsDirMode)
	if err != nil {
		return err
	}

	tw := util.TemplateWriter{
		Name:     "konnectivity-agent",
		Template: konnectivityAgentTemplate,
		Data: konnectivityAgentConfig{
			APIAddress: k.ClusterConfig.Spec.API.APIAddress(),
			Image:      k.ClusterConfig.Images.Konnectivity.URI(),
		},
		Path: filepath.Join(konnectivityDir, "konnectivity-agent.yaml"),
	}
	err = tw.Write()
	if err != nil {
		return fmt.Errorf("failed to write konnectivity agent manifest: %v", err)
	}
	return nil
}

func (k *Konnectivity) runLease() {
	k.done = make(chan bool)

	go func() {
		logrus.Infof("starting %v lease watcher", serviceName)
		leaseLock, err := k.newLeaseLock()
		if err != nil {
			logrus.Error(err)
		}
		k.leaseLock = leaseLock
		ctx := context.Background()
		for {
			select {
			case <-k.done:
				logrus.Debugf("stopping lease watcher for %v", serviceName)
				return
			default:
				k.leaseLock.LeaseRunner(ctx)
			}
		}
	}()

	go func() {
		logrus.Infof("watching %v lease holders", serviceName)
		ticker := time.NewTicker(30 * time.Second)
		ctx := context.Background()

		for {
			select {
			case <-k.done:
				logrus.Debugf("stopping lease holder count for %v", serviceName)
				return
			case <-ticker.C:
				observedLeaseHolders := k.leaseLock.CountValidLeaseHolders(ctx)
				if observedLeaseHolders != k.serverCount {
					logrus.Debugf("change in %v lease holders detected. refreshing service.", serviceName)
					k.serverCount = observedLeaseHolders

					// restarting service
					if err := k.restartService(); err != nil {
						logrus.Errorf("failed to restart %v: %v", serviceName, err)
					}
				}
			}
			logrus.Debugf("found %v lease holders for %v", k.serverCount, serviceName)
		}
	}()
}

func (k *Konnectivity) newLeaseLock() (*kubernetes.LeaseLock, error) {
	client, err := k.KubeClientFactory.GetClient()
	if err != nil {
		return nil, fmt.Errorf("failed to get lease client: %v", err)
	}

	holderIdentity, err := os.Hostname()
	if err != nil {
		return nil, err
	}

	log := logrus.WithFields(logrus.Fields{"component": "konnectivity"})

	leaseConfig := &kubernetes.LeaseConfig{
		HolderIdentity: holderIdentity,
		Name:           fmt.Sprintf("%v-%v", konnectivityLeaseName, holderIdentity),
		Namespace:      konnectivityLeaseNameSpace,
		ServiceName:    serviceName,
		LeaseDuration:  120 * time.Second, // to prevent flapping of the konnectivity service, the lease is somewhat longer than normal
		RenewDeadline:  40 * time.Second,
		RetryPeriod:    30 * time.Second,
	}
	return &kubernetes.LeaseLock{
		Config: leaseConfig,
		Client: client.CoordinationV1(),
		Log:    log,
	}, nil
}

func (k *Konnectivity) stopLease() {
	k.done <- true
	close(k.done)
}

func (k *Konnectivity) restartService() error {
	if err := k.Stop(); err != nil {
		return fmt.Errorf("failed to stop %v: %v", serviceName, err)
	}
	if err := k.Run(); err != nil {
		return fmt.Errorf("failed to start %v: %v", serviceName, err)
	}
	return nil
}

const konnectivityAgentTemplate = `
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: system:konnectivity-server
  labels:
    kubernetes.io/cluster-service: "true"
    addonmanager.kubernetes.io/mode: Reconcile
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
    addonmanager.kubernetes.io/mode: Reconcile
---
apiVersion: apps/v1
# Alternatively, you can deploy the agents as Deployments. It is not necessary
# to have an agent on each node.
kind: DaemonSet
metadata:
  labels:
    addonmanager.kubernetes.io/mode: Reconcile
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
      containers:
        - image: {{ .Image }}
          name: konnectivity-agent
          command: ["/proxy-agent"]
          args: [
                  "--logtostderr=true",
                  "--ca-cert=/var/run/secrets/kubernetes.io/serviceaccount/ca.crt",
                  # Since the konnectivity server runs with hostNetwork=true,
                  # this is the IP address of the master machine.
                  "--proxy-server-host={{ .APIAddress }}",
                  "--proxy-server-port=8132",
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

// Health-check interface
func (k *Konnectivity) Healthy() error { return nil }
