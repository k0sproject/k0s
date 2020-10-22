package server

import (
	"fmt"
	"os"
	"path"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"

	config "github.com/Mirantis/mke/pkg/apis/v1beta1"
	"github.com/Mirantis/mke/pkg/assets"
	"github.com/Mirantis/mke/pkg/constant"
	"github.com/Mirantis/mke/pkg/supervisor"
	"github.com/Mirantis/mke/pkg/util"
)

// Konnectivity implement the component interface of konnectivity server
type Konnectivity struct {
	ClusterConfig *config.ClusterConfig
	supervisor    supervisor.Supervisor
	uid           int
	gid           int
}

// Init ...
func (k *Konnectivity) Init() error {
	var err error
	k.uid, err = util.GetUID(constant.ApiserverUser)
	if err != nil {
		logrus.Warning(errors.Wrap(err, "Running konnectivity as root"))
	}

	k.gid, _ = util.GetGID(constant.Group)

	return assets.Stage(constant.BinDir, "konnectivity-server", constant.BinDirMode, constant.Group)
}

// Run ..
func (k *Konnectivity) Run() error {
	logrus.Info("Starting konnectivity")
	k.supervisor = supervisor.Supervisor{
		Name:    "konnectivity",
		BinPath: assets.BinPath("konnectivity-server"),
		Dir:     constant.DataDir,
		Args: []string{
			fmt.Sprintf("--uds-name=%s", path.Join(constant.RunDir, "konnectivity-server.sock")),
			fmt.Sprintf("--cluster-cert=%s", path.Join(constant.CertRootDir, "server.crt")),
			fmt.Sprintf("--cluster-key=%s", path.Join(constant.CertRootDir, "server.key")),
			fmt.Sprintf("--kubeconfig=%s", constant.AdminKubeconfigConfigPath), // FIXME: should have user rights
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
		},
		UID: k.uid,
		GID: k.gid,
	}

	k.supervisor.Supervise()

	return k.writeKonnectivityAgent()
}

// Stop stops
func (k *Konnectivity) Stop() error {
	return k.supervisor.Stop()
}

type konnectivityAgentConfig struct {
	APIAddress string
	Image      string
}

func (k *Konnectivity) writeKonnectivityAgent() error {
	konnectivityDir := path.Join(constant.ManifestsDir, "konnectivity")
	err := os.MkdirAll(konnectivityDir, constant.ManifestsDirMode)
	if err != nil {
		return err
	}

	tw := util.TemplateWriter{
		Name:     "konnectivity-agent",
		Template: konnectivityAgentTemplate,
		Data: konnectivityAgentConfig{
			APIAddress: k.ClusterConfig.Spec.API.Address,
			Image:      k.ClusterConfig.Images.Konnectivity.URI(),
		},
		Path: path.Join(konnectivityDir, "konnectivity-agent.yaml"),
	}
	err = tw.Write()
	if err != nil {
		return errors.Wrap(err, "failed to write konnectivity agent manifest")
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
