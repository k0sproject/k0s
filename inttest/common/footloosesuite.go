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
package common

import (
	"bytes"
	"context"
	"crypto/tls"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"path"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"text/template"
	"time"

	"github.com/go-openapi/jsonpointer"
	"github.com/stretchr/testify/suite"
	"gopkg.in/yaml.v2"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/k0sproject/k0s/internal/util"

	"github.com/weaveworks/footloose/pkg/cluster"
	"github.com/weaveworks/footloose/pkg/config"

	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	"k8s.io/apimachinery/pkg/util/wait"
)

// FootlooseSuite defines all the common stuff we need to be able to run k0s testing on footloose
type FootlooseSuite struct {
	suite.Suite

	Cluster *cluster.Cluster

	ControllerCount       int
	WorkerCount           int
	KubeAPIExternalPort   int
	K0sAPIExternalPort    int
	KonnectivityAgentPort int
	KonnectivityAdminPort int
	WithLB                bool

	ExtraVolumes  []config.Volume
	tearDownTimer *time.Timer

	footlooseConfig config.Config

	keyDir string
}

// SetupSuite does all the setup work, namely boots up footloose cluster
func (s *FootlooseSuite) SetupSuite() {
	if s.KubeAPIExternalPort == 0 {
		s.KubeAPIExternalPort = 6443
	}
	if s.K0sAPIExternalPort == 0 {
		s.K0sAPIExternalPort = 9443
	}
	if s.KonnectivityAdminPort == 0 {
		s.KonnectivityAdminPort = 8133
	}
	if s.KonnectivityAgentPort == 0 {
		s.KonnectivityAgentPort = 8132
	}
	dir, err := ioutil.TempDir("", "footloose-keys")
	if err != nil {
		s.T().Logf("ERROR: failed to load footloose config: %s", err.Error())
		s.T().FailNow()
	}
	s.keyDir = dir
	s.footlooseConfig = s.createConfig()

	suiteCluster, err := cluster.New(s.footlooseConfig)

	if err != nil {
		s.T().Logf("ERROR: failed to load footloose config: %s", err.Error())
		s.T().FailNow()
		return
	}

	// we first try to delete instances from previous runs, if they happen to exists
	_ = suiteCluster.Delete()
	err = suiteCluster.Create()
	if err != nil {
		s.FailNowf("failed to create footloose suiteCluster: %s", err.Error())
		s.T().FailNow()
		return
	}
	s.Cluster = suiteCluster
	if s.WithLB {
		go s.startHAProxy()
	}
	timeout := getTestTimeout()
	s.T().Logf("using test timeout for teardown: %s", timeout.String())
	s.tearDownTimer = time.AfterFunc(timeout, func() {
		s.TearDownSuite()
	})

	// set up signal handler so we teardown on SIGINT or SIGTERM

	c := make(chan os.Signal)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM, syscall.SIGINT)
	go func() {
		<-c
		s.TearDownSuite()
		os.Exit(1)
	}()

	s.waitForSSH()
}

func (s *FootlooseSuite) waitForSSH() {
	var err error
	// SSH through cluster should wait until we actually can get it through, but it doesn't
	for i := 0; i < 30; i++ {
		err = s.Cluster.SSH(s.ControllerNode(0), "root", "hostname")
		if err == nil {
			break
		}
		s.T().Logf("retrying ssh to %s", s.ControllerNode(0))
		time.Sleep(1 * time.Second)
	}
	if err != nil {
		s.FailNowf("failed to ssh to %s: %s", s.ControllerNode(0), err.Error())
		s.T().FailNow()
		return
	}
}

// ControllerNode gets the node name of given controller index
func (s *FootlooseSuite) ControllerNode(idx int) string {
	return fmt.Sprintf(s.footlooseConfig.Machines[0].Spec.Name, idx)
}

// WorkerNode gets the node name of given worker index
func (s *FootlooseSuite) WorkerNode(idx int) string {
	return fmt.Sprintf(s.footlooseConfig.Machines[1].Spec.Name, idx)
}

// TearDownSuite does the cleanup work, namely destroy the footloose boxes
func (s *FootlooseSuite) TearDownSuite() {
	// Make sure we don't fire the timer based teardown anymore
	s.tearDownTimer.Stop()

	if s.Cluster == nil {
		return
	}

	machines, err := s.Cluster.Inspect(nil)
	if err != nil {
		s.T().Logf("failed to inspect footloose cluster")
	}

	for _, m := range machines {
		if strings.HasPrefix(m.Hostname(), "lb") {
			continue
		}
		ssh, err := s.SSH(m.Hostname())
		if err != nil {
			s.T().Logf("failed to ssh to node %s to get logs", m.Hostname())
			continue
		}
		log, err := ssh.ExecWithOutput("cat /tmp/k0s-*.log")
		if err != nil {
			s.T().Logf("failed to cat logs on machine %s: %s", m.Hostname(), err)
		}
		logPath := path.Join("/tmp", fmt.Sprintf("%s.log", m.Hostname()))
		if err := ioutil.WriteFile(logPath, []byte(log), 0700); err != nil {
			s.T().Logf("failed to save logs from machine %s: %s", m.Hostname(), err)
		}

		s.T().Logf("wrote log of node %s to %s", m.Hostname(), logPath)
		ssh.Disconnect()
	}

	if s.keepEnvironment() {
		footlooseYaml, err := yaml.Marshal(s.footlooseConfig)
		if err != nil {
			s.T().Logf("failed to marshall footloose yaml: %s", err.Error())
			return
		}
		filename := path.Join(os.TempDir(), util.RandomString(8)+"-footloose.yaml")
		err = ioutil.WriteFile(filename, footlooseYaml, 0700)
		if err != nil {
			s.T().Logf("failed to write footloose yaml: %s", err.Error())
			return
		}
		s.T().Logf("footloose cluster left intact for debugging. Needs to be manually cleaned with: footloose delete --config %s", filename)
	} else {
		err = s.Cluster.Delete()
		if err != nil {
			s.T().Logf("failed to delete footloose cluster, we might've left some thrash around: %s", err.Error())
		}
		err = os.RemoveAll(s.keyDir)
		if err != nil {
			s.T().Logf("ERROR: failed to remove footloose keys: %s", err.Error())
		}

	}

}

const keepAfterTestsEnv = "K0S_KEEP_AFTER_TESTS"

func (s *FootlooseSuite) keepEnvironment() bool {
	keepAfterTests := os.Getenv(keepAfterTestsEnv)
	switch keepAfterTests {
	case "", "never":
		return false
	case "always":
		return true
	case "failure":
		return s.T().Failed()
	default:
		return false
	}
}

func getDataDirOpt(args []string) string {
	for _, arg := range args {
		if strings.HasPrefix(arg, "--data-dir=") {
			return arg
		}
	}
	return ""
}

func (s *FootlooseSuite) startHAProxy() {

	addresses := s.getControllersIPAddresses()
	ssh, err := s.SSH("lb0")
	s.Require().NoError(err)
	defer ssh.Disconnect()
	content := s.getLBConfig(addresses)

	_, err = ssh.ExecWithOutput(fmt.Sprintf("echo '%s' >%s", content, "/tmp/haproxy.cfg"))

	s.Require().NoError(err)
	_, err = ssh.ExecWithOutput("haproxy -c -f /tmp/haproxy.cfg")
	s.Require().NoError(err, "LB configuration is broken")
	_, err = ssh.ExecWithOutput("haproxy -f /tmp/haproxy.cfg &")
	s.Require().NoError(err, "Can't start LB")
}

func (s *FootlooseSuite) getLBConfig(adresses []string) string {
	tpl := `
defaults
    # timeouts are to prevent warning during haproxy -c call
    mode tcp
   	timeout connect 10s
    timeout client 30s
    timeout server 30s

frontend kubeapi

    bind :{{ .KubeAPIExternalPort }}
    default_backend kubeapi

frontend k0sapi
    bind :{{ .K0sAPIExternalPort }}
    default_backend k0sapi

frontend konnectivityAdmin
    bind :{{ .KonnectivityAdminPort }}
    default_backend admin


frontend konnectivityAgent
    bind :{{ .KonnectivityAgentPort }}
    default_backend agent


{{ $OUT := .}}

backend kubeapi
{{ range $addr := .IPAddresses }}
	server  {{ $addr }} {{ $addr }}:{{ $OUT.KubeAPIExternalPort }}
{{ end }}

backend k0sapi
{{ range $addr := .IPAddresses }}
	server {{ $addr }} {{ $addr }}:{{ $OUT.K0sAPIExternalPort }}
{{ end }}

backend admin
{{ range $addr := .IPAddresses }}
	server {{ $addr }} {{ $addr }}:{{ $OUT.KonnectivityAdminPort }}
{{ end }}

backend agent
{{ range $addr := .IPAddresses }}
	server {{ $addr }} {{ $addr }}:{{ $OUT.KonnectivityAgentPort }}
{{ end }}
`
	content := bytes.NewBuffer([]byte{})
	s.Assert().NoError(template.Must(template.New("haproxy").Parse(tpl)).Execute(content, struct {
		KubeAPIExternalPort   int
		K0sAPIExternalPort    int
		KonnectivityAgentPort int
		KonnectivityAdminPort int

		IPAddresses []string
	}{
		KubeAPIExternalPort:   s.KubeAPIExternalPort,
		K0sAPIExternalPort:    s.K0sAPIExternalPort,
		KonnectivityAdminPort: s.KonnectivityAdminPort,
		KonnectivityAgentPort: s.KonnectivityAgentPort,
		IPAddresses:           adresses,
	}))

	return content.String()
}

func (s *FootlooseSuite) getControllersIPAddresses() []string {
	upstreams := make([]string, s.ControllerCount)
	addresses := make([]string, s.ControllerCount)
	for i := 0; i < s.ControllerCount; i++ {
		upstreams[i] = fmt.Sprintf("controller%d", i)
	}

	machines, err := s.Cluster.Inspect(upstreams)

	s.Require().NoError(err)

	for i := 0; i < s.ControllerCount; i++ {
		addresses[i] = machines[i].Status().IP
	}
	return addresses
}

// InitController initializes a controller
func (s *FootlooseSuite) InitController(idx int, k0sArgs ...string) error {
	controllerNode := s.ControllerNode(idx)
	ssh, err := s.SSH(controllerNode)
	if err != nil {
		return err
	}
	defer ssh.Disconnect()
	// Allow any arch for etcd in smokes
	startCmd := fmt.Sprintf("ETCD_UNSUPPORTED_ARCH=%s nohup k0s controller --debug %s >/tmp/k0s-controller.log 2>&1 &", runtime.GOARCH, strings.Join(k0sArgs, " "))
	_, err = ssh.ExecWithOutput(startCmd)
	if err != nil {
		s.T().Logf("failed to execute '%s' on %s", startCmd, controllerNode)
		return err
	}

	return s.WaitForKubeAPI(controllerNode, getDataDirOpt(k0sArgs))
}

// GetJoinToken generates join token for the asked role
func (s *FootlooseSuite) GetJoinToken(role string, extraArgs ...string) (string, error) {
	// assume we have main on node 0 always
	controllerNode := s.ControllerNode(0)
	s.Contains([]string{"controller", "worker"}, role, "Bad role")
	ssh, err := s.SSH(controllerNode)

	if err != nil {
		return "", err
	}
	defer ssh.Disconnect()
	token, err := ssh.ExecWithOutput(fmt.Sprintf("k0s token create --role=%s %s", role, strings.Join(extraArgs, " ")))
	if err != nil {
		return "", fmt.Errorf("can't get join token: %v", err)
	}
	outputParts := strings.Split(token, "\n")
	// in case of no k0s.conf given, there might be warnings on the first few lines

	token = outputParts[len(outputParts)-1]
	return token, nil

}

// RunWorkers joins all the workers to the cluster
func (s *FootlooseSuite) RunWorkers(args ...string) error {
	token, err := s.GetJoinToken("worker", getDataDirOpt(args))
	if err != nil {
		return err
	}
	return s.RunWorkersWithToken(token, args...)
}

func (s *FootlooseSuite) RunWorkersWithToken(token string, args ...string) error {
	ssh, err := s.SSH("controller0")
	s.Require().NoError(err)
	defer ssh.Disconnect()
	if token == "" {
		return fmt.Errorf("got empty token for worker join")
	}
	workerCommand := fmt.Sprintf(`nohup k0s --debug worker %s "%s" >/tmp/k0s-worker.log 2>&1 &`, strings.Join(args, " "), token)

	for i := 0; i < s.WorkerCount; i++ {
		sshWorker, err := s.SSH(s.WorkerNode(i))
		if err != nil {
			return err
		}
		defer sshWorker.Disconnect()
		_, err = sshWorker.ExecWithOutput(workerCommand)
		if err != nil {
			return err
		}
	}
	return nil
}

// SSH establishes an SSH connection to the node
func (s *FootlooseSuite) SSH(node string) (*SSHConnection, error) {
	m, err := s.MachineForName(node)
	if err != nil {
		return nil, err
	}

	hostPort, err := m.HostPort(22)
	if err != nil {
		return nil, err
	}

	ssh := &SSHConnection{
		Address: "localhost", // We're always SSH'ing through port mappings
		User:    "root",
		Port:    hostPort,
		KeyPath: path.Join(s.keyDir, "id_rsa"),
	}

	err = ssh.Connect()
	if err != nil {
		return nil, err
	}

	return ssh, nil
}

// MachineForName gets the named machine details
func (s *FootlooseSuite) MachineForName(name string) (*cluster.Machine, error) {
	machines, err := s.Cluster.Inspect(nil)
	if err != nil {
		return nil, err
	}
	for _, m := range machines {
		if m.Hostname() == name {
			return m, nil
		}
	}

	return nil, fmt.Errorf("no machine found with name %s", name)
}

func (s *FootlooseSuite) StopController(name string) error {
	ssh, err := s.SSH(name)
	s.Require().NoError(err)
	defer ssh.Disconnect()
	s.T().Log("killing k0s")
	_, err = ssh.ExecWithOutput("kill $(pidof k0s) && while pidof k0s; do sleep 0.1s; done")
	return err
}

func (s *FootlooseSuite) Reset(name string) error {
	ssh, err := s.SSH(name)
	s.NoError(err)
	defer ssh.Disconnect()
	_, err = ssh.ExecWithOutput("k0s reset --debug")
	return err
}

// KubeClient return kube client by loading the admin access config from given node
func (s *FootlooseSuite) GetKubeConfig(node string, k0sKubeconfigArgs ...string) (*rest.Config, error) {
	machine, err := s.MachineForName(node)
	if err != nil {
		return nil, err
	}
	ssh, err := s.SSH(node)
	if err != nil {
		return nil, err
	}
	defer ssh.Disconnect()

	kubeConfigCmd := fmt.Sprintf("k0s kubeconfig admin %s", strings.Join(k0sKubeconfigArgs, " "))
	kubeConf, err := ssh.ExecWithOutput(kubeConfigCmd)
	if err != nil {
		return nil, err
	}
	cfg, err := clientcmd.RESTConfigFromKubeConfig([]byte(kubeConf))
	s.Require().NoError(err)

	hostURL, err := url.Parse(cfg.Host)
	if err != nil {
		return nil, fmt.Errorf("can't parse port value `%s`: %w", cfg.Host, err)
	}
	port, err := strconv.ParseInt(hostURL.Port(), 10, 32)

	if err != nil {
		return nil, fmt.Errorf("can't parse port value `%s`: %w", hostURL.Port(), err)
	}
	hostPort, err := machine.HostPort(int(port))
	if err != nil {
		return nil, fmt.Errorf("footloose machine has to have %d port mapped: %w", port, err)
	}
	cfg.Host = fmt.Sprintf("localhost:%d", hostPort)
	return cfg, nil
}

// KubeClient return kube client by loading the admin access config from given node
func (s *FootlooseSuite) KubeClient(node string, k0sKubeconfigArgs ...string) (*kubernetes.Clientset, error) {
	cfg, err := s.GetKubeConfig(node, k0sKubeconfigArgs...)
	if err != nil {
		return nil, err
	}
	return kubernetes.NewForConfig(cfg)
}

// WaitForNodeReady wait that we see the given node in "Ready" state in kubernetes API
func (s *FootlooseSuite) WaitForNodeReady(node string, kc *kubernetes.Clientset) error {
	s.T().Logf("waiting to see %s ready in kube API", node)
	return wait.PollImmediate(100*time.Millisecond, 5*time.Minute, func() (done bool, err error) {
		n, err := kc.CoreV1().Nodes().Get(context.TODO(), node, v1.GetOptions{})
		if err != nil {
			return false, nil
		}

		for _, nc := range n.Status.Conditions {
			if nc.Type == "Ready" && nc.Status == "True" {
				s.T().Logf("%s is Ready in API", node)
				return true, nil
			}
		}

		return false, nil
	})
}

// GetNodeLabels return the labels of given node
func (s *FootlooseSuite) GetNodeLabels(node string, kc *kubernetes.Clientset) (map[string]string, error) {
	n, err := kc.CoreV1().Nodes().Get(context.TODO(), node, v1.GetOptions{})
	if err != nil {
		return nil, err
	}
	return n.Labels, nil
}

// GetNodeLabels return the labels of given node
func (s *FootlooseSuite) GetNodeAnnotations(node string, kc *kubernetes.Clientset) (map[string]string, error) {
	n, err := kc.CoreV1().Nodes().Get(context.TODO(), node, v1.GetOptions{})
	if err != nil {
		return nil, err
	}
	return n.Annotations, nil
}

// AddNodeLabel adds a label to the provided node.
func (s *FootlooseSuite) AddNodeLabel(node string, kc *kubernetes.Clientset, key string, value string) (*corev1.Node, error) {
	return nodeValuePatchAdd(node, kc, "/metadata/labels", key, value)
}

// AddNodeAnnotation adds an annotation to the provided node.
func (s *FootlooseSuite) AddNodeAnnotation(node string, kc *kubernetes.Clientset, key string, value string) (*corev1.Node, error) {
	return nodeValuePatchAdd(node, kc, "/metadata/annotations", key, value)
}

// nodeValuePatchAdd patch-adds a key/value to a specific path via the Node API
func nodeValuePatchAdd(node string, kc *kubernetes.Clientset, path string, key string, value string) (*corev1.Node, error) {
	keyPath := fmt.Sprintf("%s/%s", path, jsonpointer.Escape(key))
	patch := fmt.Sprintf(`[{"op":"add", "path":"%s", "value":"%s" }]`, keyPath, value)
	return kc.CoreV1().Nodes().Patch(context.TODO(), node, types.JSONPatchType, []byte(patch), v1.PatchOptions{})
}

// WaitForKubeAPI waits until we see kube API online on given node.
// Timeouts with error return in 5 mins
func (s *FootlooseSuite) WaitForKubeAPI(node string, k0sKubeconfigArgs ...string) error {
	s.T().Logf("waiting for kube api to start on node %s", node)
	return wait.PollImmediate(100*time.Millisecond, 5*time.Minute, func() (done bool, err error) {
		kc, err := s.KubeClient(node, k0sKubeconfigArgs...)
		if err != nil {
			return false, nil
		}
		v, err := kc.ServerVersion()
		if err != nil {
			return false, nil
		}
		ctx, cancel := context.WithTimeout(context.TODO(), 5*time.Second)
		defer cancel()
		res := kc.RESTClient().Get().RequestURI("/readyz").Do(ctx)
		if res.Error() != nil {
			return false, nil
		}
		var statusCode int
		res.StatusCode(&statusCode)
		if statusCode != http.StatusOK {
			return false, nil
		}

		s.T().Logf("kube api up-and-running, version: %s", v.String())

		return true, nil
	})
}

// WaitJoinApi waits untill we see k0s join api up-and-running on a given node
// Timeouts with error return in 5 mins
func (s *FootlooseSuite) WaitJoinAPI(node string) error {
	s.T().Logf("waiting for join api to start on node %s", node)
	return wait.PollImmediate(100*time.Millisecond, 5*time.Minute, func() (done bool, err error) {
		joinAPIStatus, err := s.GetHTTPStatus(node, "/v1beta1/ca")
		if err != nil {
			return false, nil
		}
		// JoinAPI returns always un-authorized when called with no token, but it's a signal that it properly up-and-running still
		if joinAPIStatus != http.StatusUnauthorized {
			return false, nil
		}

		s.T().Logf("join api up-and-running")

		return true, nil

	})
}

func (s *FootlooseSuite) GetHTTPStatus(node string, path string) (int, error) {
	m, err := s.MachineForName(node)
	if err != nil {
		return 0, err
	}
	joinPort, err := m.HostPort(s.K0sAPIExternalPort)
	if err != nil {
		return 0, err
	}

	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	client := &http.Client{Transport: tr}
	checkURL := fmt.Sprintf("https://localhost:%d/%s", joinPort, path)
	resp, err := client.Get(checkURL)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	return resp.StatusCode, nil
}

func (s *FootlooseSuite) createConfig() config.Config {
	binPath := os.Getenv("K0S_PATH")
	if binPath == "" {
		s.FailNow("K0S_PATH env needs to be set to k0s binary")
	}

	volumes := []config.Volume{
		{
			Type:        "bind",
			Source:      "/lib/modules",
			Destination: "/lib/modules",
		},
		{
			Type:        "bind",
			Source:      binPath,
			Destination: "/usr/bin/k0s",
		},
		{
			Type:        "volume",
			Destination: "/var/lib/k0s",
		},
	}

	volumes = append(volumes, s.ExtraVolumes...)

	portMaps := []config.PortMapping{
		{
			ContainerPort: 22, // SSH
		},
		{
			ContainerPort: 10250, // kubelet logs
		},
		{
			ContainerPort: uint16(s.K0sAPIExternalPort), // kube API
		},
		{
			ContainerPort: uint16(s.KubeAPIExternalPort), // kube API
		},
		{
			ContainerPort: uint16(6060), // pprof API
		},
	}

	cfg := config.Config{
		Cluster: config.Cluster{
			Name:       s.T().Name(),
			PrivateKey: path.Join(s.keyDir, "id_rsa"),
		},
		Machines: []config.MachineReplicas{
			{
				Count: s.ControllerCount,
				Spec: config.Machine{
					Image:        "footloose-alpine",
					Name:         "controller%d",
					Privileged:   true,
					Volumes:      volumes,
					PortMappings: portMaps,
				},
			},
			{
				Count: s.WorkerCount,
				Spec: config.Machine{
					Image:        "footloose-alpine",
					Name:         "worker%d",
					Privileged:   true,
					Volumes:      volumes,
					PortMappings: portMaps,
				},
			},
		},
	}

	if s.WithLB {
		cfg.Machines = append(cfg.Machines, config.MachineReplicas{
			Spec: config.Machine{
				Name:         "lb%d",
				Image:        "footloose-alpine",
				Privileged:   true,
				Volumes:      volumes,
				PortMappings: portMaps,
				Ignite:       nil,
			},
			Count: 1,
		})
	}
	return cfg
}

// DefaultTimeout defines the default timeout for triggering custom teardown functionality
const DefaultTimeout = 9 * time.Minute // The default golang test timeout is 10mins

func getTestTimeout() time.Duration {
	for _, a := range os.Args {
		if strings.HasPrefix(a, "-test.timeout") {
			t := strings.Split(a, "=")[1]
			timeout, err := time.ParseDuration(t)
			if err != nil {
				return DefaultTimeout
			}
			// Let's shave 10% off, so k0s suite has enough time to run teardown
			testTimeout := time.Duration(float64(timeout.Milliseconds())*0.90) * time.Millisecond
			return testTimeout
		}
	}
	return DefaultTimeout
}

// GetMainIPAddress returns controller ip address
func (s *FootlooseSuite) GetControllerIPAddress(idx int) string {
	ssh, err := s.SSH(s.ControllerNode(idx))
	s.Require().NoError(err)
	defer ssh.Disconnect()

	ipAddress, err := ssh.ExecWithOutput("hostname -i")
	s.Require().NoError(err)
	return ipAddress
}
