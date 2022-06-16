/*
Copyright 2022 k0s authors

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
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"os/signal"
	"path"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"testing"
	"text/template"
	"time"

	apclient "github.com/k0sproject/k0s/pkg/apis/autopilot.k0sproject.io/v1beta2/clientset"
	extclient "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset/typed/apiextensions/v1"

	"github.com/go-openapi/jsonpointer"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/suite"
	"github.com/weaveworks/footloose/pkg/cluster"
	"github.com/weaveworks/footloose/pkg/config"
	"golang.org/x/sync/errgroup"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
	"sigs.k8s.io/yaml"

	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

const (
	controllerNodeNameFormat   = "controller%d"
	workerNodeNameFormat       = "worker%d"
	lbNodeNameFormat           = "lb%d"
	etcdNodeNameFormat         = "etcd%d"
	updateServerNodeNameFormat = "updateserver%d"

	defaultK0sBinaryFullPath = "/usr/local/bin/k0s"
	k0sBindMountFullPath     = "/dist/k0s"
)

// FootlooseSuite defines all the common stuff we need to be able to run k0s testing on footloose.
type FootlooseSuite struct {
	suite.Suite

	// Provide alternate launch functionalities as-needed.

	LaunchMode     LaunchMode
	launchDelegate *LaunchDelegate

	/* config knobs (initialized via `initializeDefaults`) */

	ControllerCount       int
	ControllerUmask       int
	ExtraVolumes          []config.Volume
	K0sFullPath           string
	K0sAPIExternalPort    int
	KonnectivityAdminPort int
	KonnectivityAgentPort int
	KubeAPIExternalPort   int
	WithExternalEtcd      bool
	WithLB                bool
	WorkerCount           int
	WithUpdateServer      bool

	/* context and cancellation */

	ctx          context.Context
	cancelFunc   context.CancelFunc
	cleanupTasks sync.WaitGroup

	/* footloose cluster setup */

	clusterDir    string
	clusterConfig config.Config
	cluster       *cluster.Cluster

	ControllerNetworks []string
	WorkerNetworks     []string
}

// initializeDefaults initializes any unset configuration knobs to their defaults.
func (s *FootlooseSuite) initializeDefaults() {
	if s.K0sFullPath == "" {
		s.K0sFullPath = defaultK0sBinaryFullPath
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
	if s.KubeAPIExternalPort == 0 {
		s.KubeAPIExternalPort = 6443
	}
	if s.LaunchMode == "" {
		s.LaunchMode = LaunchModeStandalone
	}

	switch s.LaunchMode {
	case LaunchModeStandalone:
		s.launchDelegate = s.StandaloneLaunchDelegate()
	case LaunchModeOpenRC:
		s.launchDelegate = s.OpenRCLaunchDelegate()
	default:
		s.Require().Fail("Missing launch delegate (standalone, openrc)")
	}
}

// SetupSuite does all the setup work, namely boots up footloose cluster.
func (s *FootlooseSuite) SetupSuite() {
	s.initializeDefaults()

	s.ctx, s.cancelFunc = newSuiteContext(s.T())
	if deadline, hasDeadline := s.ctx.Deadline(); hasDeadline {
		s.T().Logf("test teardown deadline: %s", deadline.String())
	} else {
		s.T().Log("test suite has no deadline")
	}

	if err := s.initializeFootlooseCluster(); err != nil {
		s.FailNow("failed to initialize footloose cluster", err)
	}

	// perform a cleanup whenever the suite's context is canceled
	s.cleanupTasks.Add(1)
	go func() {
		defer s.cleanupTasks.Done()
		<-s.ctx.Done()
		s.cleanupSuite()
	}()

	// set up signal handler so we teardown on SIGINT or SIGTERM
	c := make(chan os.Signal, 3)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM, syscall.SIGINT)
	go func() {
		<-c
		s.TearDownSuite()
		os.Exit(1)
	}()

	s.waitForSSH()

	if s.WithLB {
		go s.startHAProxy()
	}

}

// waitForSSH waits to get a SSH connection to all footloose machines defined as part of the test suite.
// Each node is tried in parallel for ~30secs max
func (s *FootlooseSuite) waitForSSH() {
	nodes := []string{}
	for i := 0; i < s.ControllerCount; i++ {
		nodes = append(nodes, s.ControllerNode(i))
	}
	for i := 0; i < s.WorkerCount; i++ {
		nodes = append(nodes, s.WorkerNode(i))
	}

	if s.WithLB {
		nodes = append(nodes, s.LBNode(0))
	}

	s.T().Logf("total of %d nodes: %v", len(nodes), nodes)

	g := errgroup.Group{}
	for _, node := range nodes {
		nodeName := node
		g.Go(func() error {
			for i := 0; i < 30; i++ {
				err := s.cluster.SSH(nodeName, "root", "hostname")
				if err == nil {
					return nil
				}
				s.T().Logf("retrying ssh to %s", nodeName)
				time.Sleep(1 * time.Second)
			}
			return fmt.Errorf("failed to get working SSH connection to %s", nodeName)
		})
	}

	err := g.Wait()
	if err != nil {
		s.FailNow("failed to ssh one or many nodes", err)
		return
	}
}

// Context returns this suite's context, which should be passed to all blocking operations.
func (s *FootlooseSuite) Context() context.Context {
	return s.ctx
}

// ControllerNode gets the node name of given controller index
func (s *FootlooseSuite) ControllerNode(idx int) string {
	return fmt.Sprintf(controllerNodeNameFormat, idx)
}

// WorkerNode gets the node name of given worker index
func (s *FootlooseSuite) WorkerNode(idx int) string {
	return fmt.Sprintf(workerNodeNameFormat, idx)
}

// LBNode gets the node of given LB index
func (s *FootlooseSuite) LBNode(idx int) string {
	if !s.WithLB {
		s.FailNow("can't get load balancer address because it's not enabled for this suite")
	}
	return fmt.Sprintf(lbNodeNameFormat, idx)
}

func (s *FootlooseSuite) ExternalEtcd(idx int) string {
	if !s.WithExternalEtcd {
		s.FailNow("can't get etcd address because it's not enabled for this suite")
	}
	return fmt.Sprintf(etcdNodeNameFormat, idx)
}

// TearDownSuite is called by testify at the very end of the suite's run.
// It cancels the suite's context in order to free the suite's resources.
func (s *FootlooseSuite) TearDownSuite() {
	s.cancelFunc()
	s.cleanupTasks.Wait()
}

// cleanupSuite does the cleanup work, namely destroy the footloose machines.
// Intended to be called after the suite's context has been canceled.
func (s *FootlooseSuite) cleanupSuite() {
	machines, err := s.InspectMachines(nil)
	if err != nil {
		s.T().Logf("failed to inspect machines")
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
		logPathInContainer := ""
		switch s.LaunchMode {
		case LaunchModeOpenRC:
			logPathInContainer = "/var/log/k0s.log"
		case LaunchModeStandalone:
			logPathInContainer = "/tmp/k0s-*.log"
		default:
			s.T().Logf(`unknown launchmode %s, dunno how to collect logs ¯\_(ツ)_/¯`, s.LaunchMode)
		}

		log, err := ssh.ExecWithOutput(fmt.Sprintf("cat %s", logPathInContainer))
		if err != nil {
			s.T().Logf("failed to cat logs on machine %s: %s", m.Hostname(), err)
		}
		logPath := path.Join("/tmp", fmt.Sprintf("%s.log", m.Hostname()))
		if err := os.WriteFile(logPath, []byte(log), 0700); err != nil {
			s.T().Logf("failed to save logs from machine %s: %s", m.Hostname(), err)
		}

		s.T().Logf("wrote log of node %s to %s", m.Hostname(), logPath)
		ssh.Disconnect()
	}

	if keepEnvironment(s.T()) {
		s.T().Logf("footloose cluster left intact for debugging; needs to be manually cleaned up with: footloose delete --config %s", path.Join(s.clusterDir, "footloose.yaml"))
	} else {
		if err := s.cluster.Delete(); err != nil {
			s.T().Logf("failed to delete footloose cluster: %v", err)
		}
		cleanupClusterDir(s.T(), s.clusterDir)
	}
}

const keepAfterTestsEnv = "K0S_KEEP_AFTER_TESTS"

func keepEnvironment(t *testing.T) bool {
	keepAfterTests := os.Getenv(keepAfterTestsEnv)
	switch keepAfterTests {
	case "", "never":
		return false
	case "always":
		return true
	case "failure":
		return t.Failed()
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
	s.Require().NoError(err, "LB configuration is broken", err)
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

listen stats
   bind *:9000
   mode http
   stats enable
   stats uri /

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

	machines, err := s.InspectMachines(upstreams)

	s.Require().NoError(err)

	for i := 0; i < s.ControllerCount; i++ {
		// If a network is supplied, the address will need to be obtained from there.
		// Note that this currently uses the first network found.
		if machines[i].Status().IP != "" {
			addresses[i] = machines[i].Status().IP
		} else if len(machines[i].Status().RuntimeNetworks) > 0 {
			addresses[i] = machines[i].Status().RuntimeNetworks[0].IP
		}
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

	if err := s.launchDelegate.InitController(ssh, k0sArgs...); err != nil {
		s.T().Logf("failed to start k0scontroller on %s: %v", controllerNode, err)
		return err
	}

	return s.WaitForKubeAPI(controllerNode, getDataDirOpt(k0sArgs))
}

// initControllerStandalone initializes a controller in 'standalone' mode, meaning that
// the k0s executable is launched directly (vs. started from a service)
func (s *FootlooseSuite) initControllerStandalone(conn *SSHConnection, k0sArgs ...string) error {
	umaskCmd := ""
	if s.ControllerUmask != 0 {
		umaskCmd = fmt.Sprintf("umask %d;", s.ControllerUmask)
	}

	// Allow any arch for etcd in smokes
	cmd := fmt.Sprintf("%s ETCD_UNSUPPORTED_ARCH=%s nohup %s controller --debug %s >/tmp/k0s-controller.log 2>&1 &", umaskCmd, runtime.GOARCH, s.K0sFullPath, strings.Join(k0sArgs, " "))

	if _, err := conn.ExecWithOutput(cmd); err != nil {
		return fmt.Errorf("unable to execute '%s': %w", cmd, err)
	}

	return nil
}

// initControllerOpenRC initializes a controller in 'openrc' mode, meaning that
// the k0s executable is launched as a service managed by openrc.
func (s *FootlooseSuite) initControllerOpenRC(conn *SSHConnection, k0sArgs ...string) error {
	if err := s.installK0sServiceOpenRC(conn, "controller"); err != nil {
		return fmt.Errorf("unable to install openrc k0s controller: %w", err)
	}

	// Configure k0s as a controller w/args
	controllerArgs := fmt.Sprintf("controller --debug %s", strings.Join(k0sArgs, " "))
	if err := configureK0sServiceArgs(conn, "controller", controllerArgs); err != nil {
		return fmt.Errorf("failed to configure k0s with '%s'", controllerArgs)
	}

	cmd := "/etc/init.d/k0scontroller start"
	if _, err := conn.ExecWithOutput(cmd); err != nil {
		return fmt.Errorf("unable to execute '%s': %w", cmd, err)
	}

	return nil
}

// installK0sServiceOpenRC will install an openrc k0s-type service (controller/worker)
// if it does not already exist.
func (s *FootlooseSuite) installK0sServiceOpenRC(conn *SSHConnection, k0sType string) error {
	existsCommand := fmt.Sprintf("/usr/bin/file /etc/init.d/k0s%s", k0sType)
	if _, err := conn.ExecWithOutput(existsCommand); err != nil {
		cmd := fmt.Sprintf("%s install %s", s.K0sFullPath, k0sType)
		if _, err := conn.ExecWithOutput(cmd); err != nil {
			return fmt.Errorf("unable to execute '%s': %w", cmd, err)
		}
	}

	return nil
}

// configureK0sServiceArgs performs some reconfiguring of the `/etc/init.d/k0s[controller|worker]`
// startup script to allow for different configurations at test time, using the same base
// image.
func configureK0sServiceArgs(conn *SSHConnection, k0sType string, args string) error {
	k0sServiceFile := fmt.Sprintf("/etc/init.d/k0s%s", k0sType)
	cmd := fmt.Sprintf("sed -i 's#^command_args=.*$#command_args=\"%s\"#g' %s", args, k0sServiceFile)

	_, err := conn.ExecWithOutput(cmd)
	if err != nil {
		return fmt.Errorf("failed to execute '%s' on %s: %w", cmd, conn.Address, err)
	}

	return nil
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

	tokenCmd := fmt.Sprintf("%s token create --role=%s %s 2>/dev/null", s.K0sFullPath, role, strings.Join(extraArgs, " "))
	token, err := ssh.ExecWithOutput(tokenCmd)
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
	for i := 0; i < s.WorkerCount; i++ {
		workerNode := s.WorkerNode(i)
		sshWorker, err := s.SSH(workerNode)
		if err != nil {
			return err
		}
		defer sshWorker.Disconnect()

		if err := s.launchDelegate.InitWorker(sshWorker, token, args...); err != nil {
			s.T().Logf("failed to start k0sworker on %s: %v", workerNode, err)
			return err
		}
	}
	return nil
}

// initWorkerStandalone initializes a worker in 'standalone' mode, meaning that
// the k0s executable is launched directly (vs. started from a service)
func (s *FootlooseSuite) initWorkerStandalone(conn *SSHConnection, token string, k0sArgs ...string) error {
	if token == "" {
		return fmt.Errorf("got empty token for worker join")
	}

	cmd := fmt.Sprintf(`nohup %s --debug worker %s "%s" >/tmp/k0s-worker.log 2>&1 &`, s.K0sFullPath, strings.Join(k0sArgs, " "), token)
	if _, err := conn.ExecWithOutput(cmd); err != nil {
		return fmt.Errorf("unable to execute '%s': %w", cmd, err)
	}

	return nil
}

// initWorkerOpenRC initializes a worker in 'openrc' mode, meaning that
// the k0s executable is launched as a service managed by openrc.
func (s *FootlooseSuite) initWorkerOpenRC(conn *SSHConnection, token string, k0sArgs ...string) error {
	if err := s.installK0sServiceOpenRC(conn, "worker"); err != nil {
		return fmt.Errorf("unable to install openrc k0s worker: %w", err)
	}

	// Configure k0s as a worker w/args
	workerArgs := fmt.Sprintf("worker --debug %s %s", strings.Join(k0sArgs, " "), token)

	if err := configureK0sServiceArgs(conn, "worker", workerArgs); err != nil {
		return fmt.Errorf("failed to configure k0s with '%s'", workerArgs)
	}

	cmd := "/etc/init.d/k0sworker start"
	if _, err := conn.ExecWithOutput(cmd); err != nil {
		return fmt.Errorf("unable to execute '%s': %w", cmd, err)
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
		KeyPath: s.clusterConfig.Cluster.PrivateKey,
	}

	err = ssh.Connect()
	if err != nil {
		return nil, err
	}

	return ssh, nil
}

func (s *FootlooseSuite) InspectMachines(hostnames []string) ([]*cluster.Machine, error) {
	return s.cluster.Inspect(hostnames)
}

// MachineForName gets the named machine details
func (s *FootlooseSuite) MachineForName(name string) (*cluster.Machine, error) {
	machines, err := s.InspectMachines(nil)
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

	return s.launchDelegate.StopController(ssh)
}

// stopControllerStandalone stops a k0s controller that was started standalone.
func (s *FootlooseSuite) stopControllerStandalone(conn *SSHConnection) error {
	stopCommand := fmt.Sprintf("kill $(pidof %s | tr \" \" \"\\n\" | sort -n | head -n1) && while pidof %s; do sleep 0.1s; done", s.K0sFullPath, s.K0sFullPath)
	if _, err := conn.ExecWithOutput(stopCommand); err != nil {
		return fmt.Errorf("unable to execute '%s': %w", stopCommand, err)
	}

	return nil
}

// stopControllerOpenRC stops a k0s controller that was started using openrc.
func (s *FootlooseSuite) stopControllerOpenRC(conn *SSHConnection) error {
	startCmd := "/etc/init.d/k0scontroller stop"
	if _, err := conn.ExecWithOutput(startCmd); err != nil {
		return fmt.Errorf("unable to execute '%s': %w", startCmd, err)
	}

	return nil
}

func (s *FootlooseSuite) Reset(name string) error {
	ssh, err := s.SSH(name)
	s.NoError(err)
	defer ssh.Disconnect()
	resetCommand := fmt.Sprintf("%s reset --debug", s.K0sFullPath)
	_, err = ssh.ExecWithOutput(resetCommand)
	return err
}

// GetKubeClientConfig returns the kubeconfig as clientcmdapi.Config struct so it can be used and loaded with clientsets directly
func (s *FootlooseSuite) GetKubeClientConfig(node string, k0sKubeconfigArgs ...string) (*clientcmdapi.Config, error) {
	machine, err := s.MachineForName(node)
	if err != nil {
		return nil, err
	}
	ssh, err := s.SSH(node)
	if err != nil {
		return nil, err
	}
	defer ssh.Disconnect()

	kubeConfigCmd := fmt.Sprintf("%s kubeconfig admin %s 2>/dev/null", s.K0sFullPath, strings.Join(k0sKubeconfigArgs, " "))
	kubeConf, err := ssh.ExecWithOutput(kubeConfigCmd)
	if err != nil {
		return nil, err
	}
	cfg, err := clientcmd.Load([]byte(kubeConf))
	s.Require().NoError(err)

	hostURL, err := url.Parse(cfg.Clusters["local"].Server)
	if err != nil {
		return nil, fmt.Errorf("can't parse port value `%s`: %w", cfg.Clusters["local"].Server, err)
	}
	port, err := strconv.ParseInt(hostURL.Port(), 10, 32)
	if err != nil {
		return nil, fmt.Errorf("can't parse port value `%s`: %w", hostURL.Port(), err)
	}
	hostPort, err := machine.HostPort(int(port))
	if err != nil {
		return nil, fmt.Errorf("footloose machine has to have %d port mapped: %w", port, err)
	}
	cfg.Clusters["local"].Server = fmt.Sprintf("https://localhost:%d", hostPort)
	return cfg, nil
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

	kubeConfigCmd := fmt.Sprintf("%s kubeconfig admin %s 2>/dev/null", s.K0sFullPath, strings.Join(k0sKubeconfigArgs, " "))
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

// CreateUserAndGetKubeClientConfig creates user and returns the kubeconfig as clientcmdapi.Config struct so it can be
// used and loaded with clientsets directly
func (s *FootlooseSuite) CreateUserAndGetKubeClientConfig(node string, username string, k0sKubeconfigArgs ...string) (*rest.Config, error) {
	machine, err := s.MachineForName(node)
	if err != nil {
		return nil, err
	}
	ssh, err := s.SSH(node)
	if err != nil {
		return nil, err
	}
	defer ssh.Disconnect()

	kubeConfigCmd := fmt.Sprintf("%s kubeconfig create %s %s 2>/dev/null", s.K0sFullPath, username, strings.Join(k0sKubeconfigArgs, " "))
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

// AutopilotClient returns a client for accessing the autopilot schema
func (s *FootlooseSuite) AutopilotClient(node string, k0sKubeconfigArgs ...string) (apclient.Interface, error) {
	cfg, err := s.GetKubeConfig(node, k0sKubeconfigArgs...)
	if err != nil {
		return nil, err
	}
	return apclient.NewForConfig(cfg)
}

// ExtensionsClient returns a client for accessing the extensions schema
func (s *FootlooseSuite) ExtensionsClient(node string, k0sKubeconfigArgs ...string) (*extclient.ApiextensionsV1Client, error) {
	cfg, err := s.GetKubeConfig(node, k0sKubeconfigArgs...)
	if err != nil {
		return nil, err
	}

	return extclient.NewForConfig(cfg)
}

// WaitForNodeReady wait that we see the given node in "Ready" state in kubernetes API
func (s *FootlooseSuite) WaitForNodeReady(node string, kc *kubernetes.Clientset) error {
	s.T().Logf("waiting to see %s ready in kube API", node)
	return Poll(s.ctx, func(ctx context.Context) (done bool, err error) {
		n, err := kc.CoreV1().Nodes().Get(ctx, node, v1.GetOptions{})
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
	n, err := kc.CoreV1().Nodes().Get(s.ctx, node, v1.GetOptions{})
	if err != nil {
		return nil, err
	}

	return n.Labels, nil
}

// WaitForNodeLabel waits for label be assigned to the node
func (s *FootlooseSuite) WaitForNodeLabel(kc *kubernetes.Clientset, node, labelKey, labelValue string) error {
	return Poll(s.ctx, func(context.Context) (done bool, err error) {
		labels, err := s.GetNodeLabels(node, kc)
		if err != nil {
			return false, nil
		}

		for k, v := range labels {
			if labelKey == k && labelValue == v {
				return true, nil
			}
		}

		return false, nil
	})
}

// GetNodeLabels return the labels of given node
func (s *FootlooseSuite) GetNodeAnnotations(node string, kc *kubernetes.Clientset) (map[string]string, error) {
	n, err := kc.CoreV1().Nodes().Get(s.ctx, node, v1.GetOptions{})
	if err != nil {
		return nil, err
	}
	return n.Annotations, nil
}

// AddNodeLabel adds a label to the provided node.
func (s *FootlooseSuite) AddNodeLabel(node string, kc *kubernetes.Clientset, key string, value string) (*corev1.Node, error) {
	return nodeValuePatchAdd(s.ctx, node, kc, "/metadata/labels", key, value)
}

// AddNodeAnnotation adds an annotation to the provided node.
func (s *FootlooseSuite) AddNodeAnnotation(node string, kc *kubernetes.Clientset, key string, value string) (*corev1.Node, error) {
	return nodeValuePatchAdd(s.ctx, node, kc, "/metadata/annotations", key, value)
}

// nodeValuePatchAdd patch-adds a key/value to a specific path via the Node API
func nodeValuePatchAdd(ctx context.Context, node string, kc *kubernetes.Clientset, path string, key string, value string) (*corev1.Node, error) {
	keyPath := fmt.Sprintf("%s/%s", path, jsonpointer.Escape(key))
	patch := fmt.Sprintf(`[{"op":"add", "path":"%s", "value":"%s" }]`, keyPath, value)
	return kc.CoreV1().Nodes().Patch(ctx, node, types.JSONPatchType, []byte(patch), v1.PatchOptions{})
}

// WaitForKubeAPI waits until we see kube API online on given node.
// Timeouts with error return in 5 mins
func (s *FootlooseSuite) WaitForKubeAPI(node string, k0sKubeconfigArgs ...string) error {
	s.T().Logf("waiting for kube api to start on node %s", node)
	return Poll(s.ctx, func(context.Context) (done bool, err error) {
		kc, err := s.KubeClient(node, k0sKubeconfigArgs...)
		if err != nil {
			s.T().Logf("kube-client error: %v", err)
			return false, nil
		}
		v, err := kc.ServerVersion()
		if err != nil {
			s.T().Logf("server version error: %v", err)
			return false, nil
		}
		ctx, cancel := context.WithTimeout(s.ctx, 5*time.Second)
		defer cancel()
		res := kc.RESTClient().Get().RequestURI("/readyz").Do(ctx)
		if res.Error() != nil {
			return false, nil
		}
		var statusCode int
		res.StatusCode(&statusCode)
		if statusCode != http.StatusOK {
			s.T().Logf("status not ok. code: %v", statusCode)
			return false, nil
		}

		s.T().Logf("kube api up-and-running, version: %s", v.String())

		return true, nil
	})
}

// WaitJoinApi waits until we see k0s join api up-and-running on a given node
// Timeouts with error return in 5 mins
func (s *FootlooseSuite) WaitJoinAPI(node string) error {
	s.T().Logf("waiting for join api to start on node %s", node)
	return Poll(s.ctx, func(context.Context) (done bool, err error) {
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

func (s *FootlooseSuite) initializeFootlooseCluster() error {
	dir, err := os.MkdirTemp("", s.T().Name()+"-footloose.")
	if err != nil {
		return errors.Wrap(err, "failed to create temporary directory for footloose configuration")
	}

	err = s.initializeFootlooseClusterInDir(dir)
	if err != nil {
		cleanupClusterDir(s.T(), dir)
	}

	return err
}

// Verifies that kubelet process has the address flag set
func (s *FootlooseSuite) GetKubeletCMDLine(node string) (string, error) {
	ssh, err := s.SSH(node)
	if err != nil {
		return "", err
	}
	defer ssh.Disconnect()

	output, err := ssh.ExecWithOutput(`cat /proc/$(pidof kubelet)/cmdline`)
	if err != nil {
		return "", err
	}

	return output, nil
}

func (s *FootlooseSuite) initializeFootlooseClusterInDir(dir string) error {
	binPath := os.Getenv("K0S_PATH")
	if binPath == "" {
		return errors.New("failed to locate k0s binary: K0S_PATH environment variable not set")
	}
	fileInfo, err := os.Stat(binPath)
	if err != nil {
		return errors.Wrapf(err, "failed to locate k0s binary %s", binPath)
	}
	if fileInfo.IsDir() {
		return errors.Errorf("failed to locate k0s binary %s: is a directory", binPath)
	}

	volumes := []config.Volume{
		{
			Type:        "bind",
			Source:      binPath,
			Destination: k0sBindMountFullPath,
			ReadOnly:    true,
		},
		{
			Type:        "volume",
			Destination: "/var/lib/k0s",
		},
	}

	// Ensure that kernel config is available in the footloose boxes.
	// See https://github.com/kubernetes/system-validators/blob/v1.6.0/validators/kernel_validator.go#L180-L190

	bindPaths := []string{
		"/usr/src/linux/.config",
		"/usr/lib/modules",
		"/lib/modules",
	}

	if kernelVersion, err := exec.Command("uname", "-r").Output(); err == nil {
		kernelVersion := strings.TrimSpace(string(kernelVersion))
		bindPaths = append(bindPaths, []string{
			"/boot/config-" + kernelVersion,
			"/usr/src/linux-" + kernelVersion,
			"/usr/lib/ostree-boot/config-" + kernelVersion,
			"/usr/lib/kernel/config-" + kernelVersion,
			"/usr/src/linux-headers-" + kernelVersion,
		}...)
	} else {
		s.T().Logf("not mounting any kernel-specific paths: %v", err)
	}

	for _, path := range bindPaths {
		if _, err := os.Stat(path); err != nil {
			continue
		}
		volumes = append(volumes, config.Volume{
			Type:        "bind",
			Source:      path,
			Destination: path,
			ReadOnly:    true,
		})
	}

	volumes = append(volumes, s.ExtraVolumes...)

	s.T().Logf("mounting volumes: %v", volumes)

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
			PrivateKey: path.Join(dir, "id_rsa"),
		},
		Machines: []config.MachineReplicas{
			{
				Count: s.ControllerCount,
				Spec: config.Machine{
					Image:        "footloose-alpine",
					Name:         controllerNodeNameFormat,
					Privileged:   true,
					Volumes:      volumes,
					PortMappings: portMaps,
					Networks:     s.ControllerNetworks,
				},
			},
			{
				Count: s.WorkerCount,
				Spec: config.Machine{
					Image:        "footloose-alpine",
					Name:         workerNodeNameFormat,
					Privileged:   true,
					Volumes:      volumes,
					PortMappings: portMaps,
					Networks:     s.WorkerNetworks,
				},
			},
		},
	}

	if s.WithLB {
		cfg.Machines = append(cfg.Machines, config.MachineReplicas{
			Spec: config.Machine{
				Name:         lbNodeNameFormat,
				Image:        "footloose-alpine",
				Privileged:   true,
				Volumes:      volumes,
				PortMappings: portMaps,
				Ignite:       nil,
				Networks:     s.ControllerNetworks,
			},
			Count: 1,
		})
	}

	if s.WithExternalEtcd {
		cfg.Machines = append(cfg.Machines, config.MachineReplicas{
			Spec: config.Machine{
				Name:         etcdNodeNameFormat,
				Image:        "footloose-alpine",
				Privileged:   true,
				PortMappings: []config.PortMapping{{ContainerPort: 22}},
			},
			Count: 1,
		})
	}

	if s.WithUpdateServer {
		cfg.Machines = append(cfg.Machines, config.MachineReplicas{
			Spec: config.Machine{
				Name:       updateServerNodeNameFormat,
				Image:      "update-server",
				Privileged: true,
				PortMappings: []config.PortMapping{
					{
						ContainerPort: 22, // SSH
					},
					{
						ContainerPort: 80,
					},
				},
				Networks: s.ControllerNetworks,
			},
			Count: 1,
		})
	}

	footlooseYaml, err := yaml.Marshal(cfg)
	if err != nil {
		return errors.Wrap(err, "failed to marshal footloose configuration")
	}

	if err = os.WriteFile(path.Join(dir, "footloose.yaml"), footlooseYaml, 0700); err != nil {
		return errors.Wrap(err, "failed to write footloose configuration to file")
	}

	cluster, err := cluster.New(cfg)
	if err != nil {
		return errors.Wrap(err, "failed to setup a new footloose cluster")
	}

	// we first try to delete instances from previous runs, if they happen to exist
	_ = cluster.Delete()
	if err := cluster.Create(); err != nil {
		return errors.Wrap(err, "failed to create footloose cluster")
	}

	s.clusterDir = dir
	s.clusterConfig = cfg
	s.cluster = cluster
	return nil
}

func cleanupClusterDir(t *testing.T, dir string) {
	if err := os.RemoveAll(dir); err != nil {
		t.Logf("failed to remove footloose configuration directory %s: %v", dir, err)
	}
}

func newSuiteContext(t *testing.T) (context.Context, context.CancelFunc) {
	// We need to reserve some time to conduct a proper teardown of the suite before the test timeout kicks in.
	if deadline, hasDeadline := t.Deadline(); hasDeadline {
		remainingTestDuration := time.Until(deadline)
		//  Let's reserve 10% ...
		reservedTeardownDuration := time.Duration(float64(remainingTestDuration.Milliseconds())*0.10) * time.Millisecond
		// ... but at least 20 seconds.
		reservedTeardownDuration = time.Duration(math.Max(float64(20*time.Second), float64(reservedTeardownDuration)))
		// And construct the context accordingly
		return context.WithDeadline(context.Background(), deadline.Add(-reservedTeardownDuration))
	}

	return context.WithCancel(context.Background())
}

// GetControllerIPAddress returns controller ip address
func (s *FootlooseSuite) GetControllerIPAddress(idx int) string {
	return s.getIPAddress(s.ControllerNode(idx))
}

func (s *FootlooseSuite) GetLBAddress() string {
	return s.getIPAddress(s.LBNode(0))
}

func (s *FootlooseSuite) GetExternalEtcdIPAddress() string {
	return s.getIPAddress(s.ExternalEtcd(0))
}

func (s *FootlooseSuite) getIPAddress(nodeName string) string {
	ssh, err := s.SSH(nodeName)
	s.Require().NoError(err)
	defer ssh.Disconnect()

	ipAddress, err := ssh.ExecWithOutput("hostname -i")
	s.Require().NoError(err)
	return ipAddress
}

type LaunchMode string

const (
	LaunchModeStandalone LaunchMode = "standalone"
	LaunchModeOpenRC     LaunchMode = "openrc"
)

// LaunchDelegate provides an indirection to the launch operations in footloosesuite
// so that alternate behaviour can be performed.
type LaunchDelegate struct {
	InitController func(conn *SSHConnection, k0sArgs ...string) error
	StopController func(conn *SSHConnection) error
	InitWorker     func(conn *SSHConnection, token string, k0sArgs ...string) error
}

// StandaloneLaunchDelegate creates a footloosesuite LaunchDelegate that starts controllers/workers
// in a 'standalone' mode, ie. not run from a service.
func (s *FootlooseSuite) StandaloneLaunchDelegate() *LaunchDelegate {
	return &LaunchDelegate{
		InitController: func(conn *SSHConnection, k0sArgs ...string) error {
			return s.initControllerStandalone(conn, k0sArgs...)
		},
		StopController: func(conn *SSHConnection) error {
			return s.stopControllerStandalone(conn)
		},
		InitWorker: func(conn *SSHConnection, token string, k0sArgs ...string) error {
			return s.initWorkerStandalone(conn, token, k0sArgs...)
		},
	}
}

// OpenRCLaunchDelegate creates a footloosesuite LaunchDelegate that starts controllers/workers
// via an openrc service.
func (s *FootlooseSuite) OpenRCLaunchDelegate() *LaunchDelegate {
	return &LaunchDelegate{
		InitController: func(conn *SSHConnection, k0sArgs ...string) error {
			return s.initControllerOpenRC(conn, k0sArgs...)
		},
		StopController: func(conn *SSHConnection) error {
			return s.stopControllerOpenRC(conn)
		},
		InitWorker: func(conn *SSHConnection, token string, k0sArgs ...string) error {
			return s.initWorkerOpenRC(conn, token, k0sArgs...)
		},
	}
}

// CreateNetwork creates a docker network with the provided name, destroying
// any network that has the same name first.
func (s *FootlooseSuite) CreateNetwork(name string) error {
	_ = s.DestroyNetwork(name)

	cmd := exec.Command("/usr/bin/docker", "network", "create", name)
	return cmd.Run()
}

// DestroyNetwork removes a docker network with the provided name.
func (s *FootlooseSuite) DestroyNetwork(name string) error {
	cmd := exec.Command("/usr/bin/docker", "network", "rm", name)
	return cmd.Run()
}

// RunCommandController runs a command via SSH on a specified controller node
func (s *FootlooseSuite) RunCommandController(idx int, command string) (string, error) {
	ssh, err := s.SSH(s.ControllerNode(idx))
	s.Require().NoError(err)
	defer ssh.Disconnect()

	return ssh.ExecWithOutput(command)
}

// GetK0sVersion returns the `k0s version` output from a specific node.
func (s *FootlooseSuite) GetK0sVersion(node string) (string, error) {
	ssh, err := s.SSH(node)
	if err != nil {
		return "", err
	}
	defer ssh.Disconnect()

	version, err := ssh.ExecWithOutput("/usr/local/bin/k0s version")
	if err != nil {
		return "", err
	}

	return version, nil
}

// GetMembers returns all of the known etcd members for a given node
func (s *FootlooseSuite) GetMembers(idx int) map[string]string {
	// our etcd instances doesn't listen on public IP, so test is performed by calling CLI tools over ssh
	// which in general even makes sense, we can test tooling as well
	sshCon, err := s.SSH(s.ControllerNode(idx))
	s.NoError(err)
	defer sshCon.Disconnect()
	output, err := sshCon.ExecWithOutput("/usr/local/bin/k0s etcd member-list")
	output = lastLine(output)
	s.NoError(err)

	members := struct {
		Members map[string]string `json:"members"`
	}{}

	err = json.Unmarshal([]byte(output), &members)
	s.NoError(err, err)

	return members.Members
}

func lastLine(text string) string {
	if text == "" {
		return ""
	}
	parts := strings.Split(text, "\n")
	return parts[len(parts)-1]
}

// WaitForSSH ensures that an SSH connection can be successfully obtained, and retries
// for up to a specific timeout/delay.
func (s *FootlooseSuite) WaitForSSH(node string, timeout time.Duration, delay time.Duration) error {
	s.T().Logf("Waiting for SSH connection to '%s'", node)
	for start := time.Now(); time.Since(start) < timeout; {
		if conn, err := s.SSH(node); err == nil {
			conn.Disconnect()
			return nil
		}

		s.T().Logf("Unable to SSH to '%s', waiting %v for retry", node, delay)
		time.Sleep(delay)
	}

	return fmt.Errorf("timed out waiting for ssh connection to '%s'", node)
}

// GetUpdateServerIPAddress returns the load balancers ip address
func (s *FootlooseSuite) GetUpdateServerIPAddress() string {
	ssh, err := s.SSH("updateserver0")
	s.Require().NoError(err)
	defer ssh.Disconnect()

	ipAddress, err := ssh.ExecWithOutput("hostname -i")
	s.Require().NoError(err)
	return ipAddress
}
