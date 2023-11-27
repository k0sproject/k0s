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

package common

import (
	"bufio"
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"os/signal"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"testing"
	"text/template"
	"time"

	"github.com/k0sproject/k0s/internal/pkg/file"
	apclient "github.com/k0sproject/k0s/pkg/client/clientset"
	"github.com/k0sproject/k0s/pkg/constant"
	"github.com/k0sproject/k0s/pkg/k0scontext"
	"github.com/k0sproject/k0s/pkg/kubernetes/watch"
	extclient "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset/typed/apiextensions/v1"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"sigs.k8s.io/yaml"

	"github.com/k0sproject/bootloose/pkg/cluster"
	"github.com/k0sproject/bootloose/pkg/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"go.uber.org/multierr"
	"golang.org/x/sync/errgroup"
)

const (
	controllerNodeNameFormat   = "controller%d"
	workerNodeNameFormat       = "worker%d"
	k0smotronNodeNameFormat    = "k0smotron%d"
	lbNodeNameFormat           = "lb%d"
	etcdNodeNameFormat         = "etcd%d"
	updateServerNodeNameFormat = "updateserver%d"

	defaultK0sBinaryFullPath = "/usr/local/bin/k0s"
	k0sBindMountFullPath     = "/dist/k0s"
	k0sNewBindMountFullPath  = "/dist/k0s-new"

	defaultK0sUpdateVersion = "v0.0.0"

	defaultBootLooseImage = "bootloose-alpine"
)

// BootlooseSuite defines all the common stuff we need to be able to run k0s testing on bootloose.
type BootlooseSuite struct {
	suite.Suite

	/* config knobs (initialized via `initializeDefaults`) */

	LaunchMode                      LaunchMode
	ControllerCount                 int
	ControllerUmask                 int
	ExtraVolumes                    []config.Volume
	K0sFullPath                     string
	AirgapImageBundleMountPoints    []string
	K0smotronImageBundleMountPoints []string
	K0sAPIExternalPort              int
	KonnectivityAdminPort           int
	KonnectivityAgentPort           int
	KubeAPIExternalPort             int
	WithExternalEtcd                bool
	WithLB                          bool
	WorkerCount                     int
	K0smotronWorkerCount            int
	WithUpdateServer                bool
	K0sUpdateVersion                string
	BootLooseImage                  string

	ctx      context.Context
	tearDown func()

	/* bootloose cluster setup */

	clusterDir     string
	clusterConfig  config.Config
	cluster        *cluster.Cluster
	launchDelegate launchDelegate

	dataDirOpt string // Data directory option of first controller, required to fetch the cluster state
}

// initializeDefaults initializes any unset configuration knobs to their defaults.
func (s *BootlooseSuite) initializeDefaults() {
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

	s.K0sUpdateVersion = os.Getenv("K0S_UPDATE_TO_VERSION")
	if s.K0sUpdateVersion == "" {
		s.K0sUpdateVersion = defaultK0sUpdateVersion
	}

	switch s.LaunchMode {
	case LaunchModeStandalone:
		s.launchDelegate = &standaloneLaunchDelegate{s.K0sFullPath, s.ControllerUmask}
	case LaunchModeOpenRC:
		s.launchDelegate = &openRCLaunchDelegate{s.K0sFullPath}
	default:
		s.Require().Fail("Unknown launch mode", s.LaunchMode)
	}

	s.BootLooseImage = os.Getenv("BOOTLOOSE_IMAGE")
	if s.BootLooseImage == "" {
		s.BootLooseImage = defaultBootLooseImage
	}
}

// SetupSuite does all the setup work, namely boots up bootloose cluster.
func (s *BootlooseSuite) SetupSuite() {
	t := s.T()

	s.initializeDefaults()

	tornDown := errors.New("suite torn down")

	var cleanupTasks sync.WaitGroup
	ctx, cancel := newSuiteContext(t)

	t.Cleanup(func() {
		cancel(errors.New("test cleanup"))
		cleanupTasks.Wait()
	})

	s.ctx, s.tearDown = ctx, func() {
		cancel(tornDown)
		cleanupTasks.Wait()
	}

	if deadline, hasDeadline := ctx.Deadline(); hasDeadline {
		t.Logf("test suite deadline: %s", deadline)
	} else {
		t.Log("test suite has no deadline")
	}

	if err := s.initializeBootlooseCluster(); err != nil {
		s.FailNow("failed to initialize bootloose cluster", err)
	}

	// perform a cleanup whenever the suite's context is canceled
	cleanupTasks.Add(1)
	go func() {
		defer cleanupTasks.Done()
		<-ctx.Done()
		// Record a test failure when the context has been canceled other than
		// through the test tear down itself. This is to ensure that the test is
		// actually marked as failed and the cluster state will be recorded.
		if err := context.Cause(ctx); !errors.Is(err, tornDown) {
			assert.Failf(t, "Test suite not properly torn down", "%v", err)
		}

		t.Logf("Cleaning up")

		// Get a fresh context for the cleanup tasks.
		ctx, cancel := signalAwareCtx(context.Background())
		defer cancel(nil)
		s.cleanupSuite(ctx, t)
	}()

	s.waitForSSH(ctx)

	if s.WithLB {
		s.startHAProxy()
	}
}

func signalAwareCtx(parent context.Context) (context.Context, context.CancelCauseFunc) {
	ctx, cancel := context.WithCancelCause(parent)

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, os.Interrupt, syscall.SIGTERM)
	go func() {
		defer signal.Stop(sigs)
		select {
		case <-ctx.Done():
		case sig := <-sigs:
			cancel(fmt.Errorf("signal received: %s", sig))
		}
	}()

	return ctx, cancel
}

// waitForSSH waits to get a SSH connection to all bootloose machines defined as part of the test suite.
// Each node is tried in parallel for ~30secs max
func (s *BootlooseSuite) waitForSSH(ctx context.Context) {
	nodes := []string{}
	for i := 0; i < s.ControllerCount; i++ {
		nodes = append(nodes, s.ControllerNode(i))
	}
	for i := 0; i < s.WorkerCount; i++ {
		nodes = append(nodes, s.WorkerNode(i))
	}
	for i := 0; i < s.K0smotronWorkerCount; i++ {
		nodes = append(nodes, s.K0smotronNode(i))
	}
	if s.WithLB {
		nodes = append(nodes, s.LBNode())
	}

	s.T().Logf("Waiting for SSH connections to %d nodes: %v", len(nodes), nodes)

	g, ctx := errgroup.WithContext(ctx)
	for _, node := range nodes {
		nodeName := node
		g.Go(func() error {
			return wait.PollUntilWithContext(ctx, 1*time.Second, func(ctx context.Context) (bool, error) {
				ssh, err := s.SSH(ctx, nodeName)
				if err != nil {
					return false, nil
				}
				defer ssh.Disconnect()

				err = ssh.Exec(ctx, "hostname", SSHStreams{})
				if err != nil {
					return false, nil
				}

				s.T().Logf("SSH connection to %s successful", nodeName)
				return true, nil
			})
		})
	}

	s.Require().NoError(g.Wait(), "Failed to ssh into all nodes")
}

// Context returns this suite's context, which should be passed to all blocking
// operations. It captures the current test as a context value, so that it can
// be retrieved by helper methods later on.
//
// Context should only be called once at the beginning of a test function, and
// then be passed along to all subsequently called functions in the usual way:
// as the first function parameter. The test framework itself doesn't have any
// means of smuggling a context to test functions, hence the suite needs to
// store it as a field, which is usually considered bad practice. However,
// relying on the context being passed along implicitly makes the test suite a
// bit edgy concerning API design and cancellation. This is the main reason why
// the suite context is being replaced during the suite cleanup: Some functions
// will obtain a context from the suite again, instead of taking it as their
// first parameter. That replacement should become obsolete once all functions
// will take the context as parameter.
func (s *BootlooseSuite) Context() context.Context {
	ctx, t := s.ctx, s.T()
	require.NotNil(t, ctx, "No suite context installed")
	if t == nil {
		return ctx
	}

	return k0scontext.WithValue(ctx, t)
}

// ControllerNode gets the node name of given controller index
func (s *BootlooseSuite) ControllerNode(idx int) string {
	return fmt.Sprintf(controllerNodeNameFormat, idx)
}

// WorkerNode gets the node name of given worker index
func (s *BootlooseSuite) WorkerNode(idx int) string {
	return fmt.Sprintf(workerNodeNameFormat, idx)
}

// K0smotronNode gets the node name of given K0smotron node index
func (s *BootlooseSuite) K0smotronNode(idx int) string {
	return fmt.Sprintf(k0smotronNodeNameFormat, idx)
}

func (s *BootlooseSuite) LBNode() string {
	if !s.WithLB {
		s.FailNow("can't get load balancer node name because it's not enabled for this suite")
	}
	return fmt.Sprintf(lbNodeNameFormat, 0)
}

func (s *BootlooseSuite) ExternalEtcdNode() string {
	if !s.WithExternalEtcd {
		s.FailNow("can't get external node name because it's not enabled for this suite")
	}
	return fmt.Sprintf(etcdNodeNameFormat, 0)
}

// StartMachines starts specific machines(s) in cluster
func (s *BootlooseSuite) Start(machineNames []string) error {
	return s.cluster.Start(machineNames)
}

// Stop stops the machines in cluster.
func (s *BootlooseSuite) Stop(machineNames []string) error {
	return s.cluster.Stop(machineNames)
}

// TearDownSuite is called by testify at the very end of the suite's run.
// It cancels the suite's context in order to free the suite's resources.
func (s *BootlooseSuite) TearDownSuite() {
	tearDown := s.tearDown
	if s.NotNil(tearDown, "Failed to tear down suite") {
		tearDown()
	}
}

// cleanupSuite does the cleanup work, namely destroy the bootloose machines.
// Intended to be called after the suite's context has been canceled.
func (s *BootlooseSuite) cleanupSuite(ctx context.Context, t *testing.T) {
	if t.Failed() {
		var wg sync.WaitGroup
		tmpDir := os.TempDir()

		if s.ControllerCount > 0 {
			wg.Add(1)
			go func() {
				defer wg.Done()
				s.collectTroubleshootSupportBundle(ctx, t, filepath.Join(tmpDir, "support-bundle.tar.gz"))
			}()
		}

		machines, err := s.InspectMachines(nil)
		if err != nil {
			t.Logf("Failed to inspect machines: %s", err.Error())
			machines = nil
		}

		for _, m := range machines {
			node := m.Hostname()
			if strings.HasPrefix(node, "lb") {
				continue
			}

			wg.Add(1)
			go func() {
				defer wg.Done()
				s.dumpNodeLogs(ctx, t, node, tmpDir)
			}()
		}
		wg.Wait()
	}

	if keepEnvironment(t) {
		t.Logf("bootloose cluster left intact for debugging; needs to be manually cleaned up with: bootloose delete --config %s", path.Join(s.clusterDir, "bootloose.yaml"))
		return
	}

	if err := s.cluster.Delete(); err != nil {
		t.Logf("Failed to delete bootloose cluster: %s", err.Error())
	}

	cleanupClusterDir(t, s.clusterDir)
}

func (s *BootlooseSuite) collectTroubleshootSupportBundle(ctx context.Context, t *testing.T, filePath string) {
	dataDir := constant.DataDirDefault
	if s.dataDirOpt != "" {
		dataDir = s.dataDirOpt[len(dataDirOptPrefix):]
	}
	cmd := fmt.Sprintf("troubleshoot-k0s-inttest.sh %q", dataDir)

	node := s.ControllerNode(0)
	ssh, err := s.SSH(ctx, node)
	if err != nil {
		t.Logf("Failed to ssh into %s to collect support bundle: %s", node, err.Error())
		return
	}
	defer ssh.Disconnect()

	err = file.WriteAtomically(filePath, 0644, func(file io.Writer) error {
		stdout := bufio.NewWriter(file)
		err := ssh.Exec(ctx, cmd, SSHStreams{Out: stdout})
		if err != nil {
			return err
		}
		return stdout.Flush()
	})
	if err != nil {
		t.Logf("Failed to collect troubleshoot support bundle on %s into %s using %q: %s", node, filePath, cmd, err.Error())
		return
	}

	t.Logf("Collected troubleshoot support bundle on %s into %s", node, filePath)
}

func (s *BootlooseSuite) dumpNodeLogs(ctx context.Context, t *testing.T, node, dir string) {
	ssh, err := s.SSH(ctx, node)
	if err != nil {
		t.Logf("Failed to ssh into %s to get logs: %s", node, err.Error())
		return
	}
	defer ssh.Disconnect()

	outPath := filepath.Join(dir, fmt.Sprintf("%s.out.log", node))
	errPath := filepath.Join(dir, fmt.Sprintf("%s.err.log", node))

	err = func() (err error) {
		type log struct {
			path   string
			writer io.Writer
		}

		outLog, errLog := log{path: outPath}, log{path: errPath}
		for _, log := range []*log{&outLog, &errLog} {
			file, err := os.Create(log.path)
			if err != nil {
				t.Logf("Failed to create log file: %s", err.Error())
				continue
			}

			defer multierr.AppendInvoke(&err, multierr.Close(file))
			buf := bufio.NewWriter(file)
			defer func() {
				if err == nil {
					err = buf.Flush()
				}
			}()
			log.writer = buf
		}

		return s.launchDelegate.ReadK0sLogs(ctx, ssh, outLog.writer, errLog.writer)
	}()
	if err != nil {
		t.Logf("Failed to collect k0s logs from %s: %s", node, err.Error())
	}

	nonEmptyPaths := make([]string, 0, 2)
	for _, path := range []string{outPath, errPath} {
		stat, err := os.Stat(path)
		if err != nil {
			continue
		}
		if stat.Size() == 0 {
			_ = os.Remove(path)
			continue
		}

		nonEmptyPaths = append(nonEmptyPaths, path)
	}

	if len(nonEmptyPaths) > 0 {
		t.Logf("Collected k0s logs from %s into %s", node, strings.Join(nonEmptyPaths, " and "))
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

const dataDirOptPrefix = "--data-dir="

func getDataDirOpt(args []string) string {
	for _, arg := range args {
		if strings.HasPrefix(arg, dataDirOptPrefix) {
			return arg
		}
	}
	return ""
}

func (s *BootlooseSuite) startHAProxy() {
	addresses := s.getControllersIPAddresses()
	ssh, err := s.SSH(s.Context(), s.LBNode())
	s.Require().NoError(err)
	defer ssh.Disconnect()
	content := s.getLBConfig(addresses)

	_, err = ssh.ExecWithOutput(s.Context(), fmt.Sprintf("echo '%s' >%s", content, "/tmp/haproxy.cfg"))

	s.Require().NoError(err)
	_, err = ssh.ExecWithOutput(s.Context(), "haproxy -c -f /tmp/haproxy.cfg")
	s.Require().NoError(err, "LB configuration is broken", err)
	_, err = ssh.ExecWithOutput(s.Context(), "haproxy -D -f /tmp/haproxy.cfg")
	s.Require().NoError(err, "Can't start LB")
}

func (s *BootlooseSuite) getLBConfig(adresses []string) string {
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

func (s *BootlooseSuite) getControllersIPAddresses() []string {
	upstreams := make([]string, s.ControllerCount)
	addresses := make([]string, s.ControllerCount)
	for i := 0; i < s.ControllerCount; i++ {
		upstreams[i] = fmt.Sprintf("controller%d", i)
	}

	machines, err := s.InspectMachines(upstreams)

	s.Require().NoError(err)

	for i := 0; i < s.ControllerCount; i++ {
		addresses[i] = machines[i].Status().IP
	}
	return addresses
}

// InitController initializes a controller
func (s *BootlooseSuite) InitController(idx int, k0sArgs ...string) error {
	controllerNode := s.ControllerNode(idx)
	ssh, err := s.SSH(s.Context(), controllerNode)
	if err != nil {
		return err
	}
	defer ssh.Disconnect()

	if err := s.launchDelegate.InitController(s.Context(), ssh, k0sArgs...); err != nil {
		s.T().Logf("failed to start k0scontroller on %s: %v", controllerNode, err)
		return err
	}

	dataDirOpt := getDataDirOpt(k0sArgs)
	if idx == 0 {
		s.dataDirOpt = dataDirOpt
	}

	return s.WaitForKubeAPI(controllerNode, dataDirOpt)
}

// GetJoinToken generates join token for the asked role
func (s *BootlooseSuite) GetJoinToken(role string, extraArgs ...string) (string, error) {
	// assume we have main on node 0 always
	controllerNode := s.ControllerNode(0)
	s.Contains([]string{"controller", "worker"}, role, "Bad role")
	ssh, err := s.SSH(s.Context(), controllerNode)
	if err != nil {
		return "", err
	}
	defer ssh.Disconnect()

	tokenCmd := fmt.Sprintf("%s token create --role=%s %s 2>/dev/null", s.K0sFullPath, role, strings.Join(extraArgs, " "))
	token, err := ssh.ExecWithOutput(s.Context(), tokenCmd)
	if err != nil {
		return "", fmt.Errorf("can't get join token: %v", err)
	}
	outputParts := strings.Split(token, "\n")
	// in case of no k0s.conf given, there might be warnings on the first few lines

	token = outputParts[len(outputParts)-1]
	return token, nil
}

// ImportK0smotrtonImages imports
func (s *BootlooseSuite) ImportK0smotronImages(ctx context.Context) error {
	for i := 0; i < s.WorkerCount; i++ {
		workerNode := s.WorkerNode(i)
		s.T().Logf("Importing images in %s", workerNode)
		sshWorker, err := s.SSH(s.Context(), workerNode)
		if err != nil {
			return err
		}
		defer sshWorker.Disconnect()

		_, err = sshWorker.ExecWithOutput(ctx, fmt.Sprintf("k0s ctr images import %s", s.K0smotronImageBundleMountPoints[0]))
		if err != nil {
			return fmt.Errorf("failed to import k0smotron images: %v", err)
		}
	}
	return nil
}

// RunWorkers joins all the workers to the cluster
func (s *BootlooseSuite) RunWorkers(args ...string) error {
	token, err := s.GetJoinToken("worker", getDataDirOpt(args))
	if err != nil {
		return err
	}
	return s.RunWorkersWithToken(token, args...)
}

// RunWorkersWithToken joins all the workers to the cluster with the given token
func (s *BootlooseSuite) RunWorkersWithToken(token string, args ...string) error {
	for i := 0; i < s.WorkerCount; i++ {
		err := s.RunWithToken(s.WorkerNode(i), token, args...)
		if err != nil {
			return err
		}
	}
	return nil
}

// RunWithToken joins a worker node to the cluster with the given token
func (s *BootlooseSuite) RunWithToken(worker string, token string, args ...string) error {
	sshWorker, err := s.SSH(s.Context(), worker)
	if err != nil {
		return err
	}
	defer sshWorker.Disconnect()

	if err := s.launchDelegate.InitWorker(s.Context(), sshWorker, token, args...); err != nil {
		s.T().Logf("failed to start k0sworker on %s: %v", worker, err)
		return err
	}
	return nil
}

// SSH establishes an SSH connection to the node
func (s *BootlooseSuite) SSH(ctx context.Context, node string) (*SSHConnection, error) {
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

	err = ssh.Connect(ctx)
	if err != nil {
		return nil, err
	}

	return ssh, nil
}

func (s *BootlooseSuite) InspectMachines(hostnames []string) ([]*cluster.Machine, error) {
	return s.cluster.Inspect(hostnames)
}

// MachineForName gets the named machine details
func (s *BootlooseSuite) MachineForName(name string) (*cluster.Machine, error) {
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

func (s *BootlooseSuite) StopController(name string) error {
	ssh, err := s.SSH(s.Context(), name)
	s.Require().NoError(err)
	defer ssh.Disconnect()
	s.T().Log("killing k0s")

	return s.launchDelegate.StopController(s.Context(), ssh)
}

func (s *BootlooseSuite) StartController(name string) error {
	ssh, err := s.SSH(s.Context(), name)
	s.Require().NoError(err)
	defer ssh.Disconnect()
	return s.launchDelegate.StartController(s.Context(), ssh)
}

func (s *BootlooseSuite) StartWorker(name string) error {
	ssh, err := s.SSH(s.Context(), name)
	s.Require().NoError(err)
	defer ssh.Disconnect()
	return s.launchDelegate.StartWorker(s.Context(), ssh)
}

func (s *BootlooseSuite) StopWorker(name string) error {
	ssh, err := s.SSH(s.Context(), name)
	s.Require().NoError(err)
	defer ssh.Disconnect()
	return s.launchDelegate.StopWorker(s.Context(), ssh)
}

func (s *BootlooseSuite) Reset(name string) error {
	ssh, err := s.SSH(s.Context(), name)
	s.Require().NoError(err)
	defer ssh.Disconnect()
	resetCommand := fmt.Sprintf("%s reset --debug", s.K0sFullPath)
	_, err = ssh.ExecWithOutput(s.Context(), resetCommand)
	return err
}

// KubeClient return kube client by loading the admin access config from given node
func (s *BootlooseSuite) GetKubeConfig(node string, k0sKubeconfigArgs ...string) (*rest.Config, error) {
	machine, err := s.MachineForName(node)
	if err != nil {
		return nil, err
	}
	ssh, err := s.SSH(s.Context(), node)
	if err != nil {
		return nil, err
	}
	defer ssh.Disconnect()

	kubeConfigCmd := fmt.Sprintf("%s kubeconfig admin %s", s.K0sFullPath, strings.Join(k0sKubeconfigArgs, " "))
	kubeConf, err := ssh.ExecWithOutput(s.Context(), kubeConfigCmd)
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
		return nil, fmt.Errorf("bootloose machine has to have %d port mapped: %w", port, err)
	}
	cfg.Host = fmt.Sprintf("localhost:%d", hostPort)
	return cfg, nil
}

// CreateUserAndGetKubeClientConfig creates user and returns the kubeconfig as clientcmdapi.Config struct so it can be
// used and loaded with clientsets directly
func (s *BootlooseSuite) CreateUserAndGetKubeClientConfig(node string, username string, k0sKubeconfigArgs ...string) (*rest.Config, error) {
	machine, err := s.MachineForName(node)
	if err != nil {
		return nil, err
	}
	ssh, err := s.SSH(s.Context(), node)
	if err != nil {
		return nil, err
	}
	defer ssh.Disconnect()

	kubeConfigCmd := fmt.Sprintf("%s kubeconfig create %s %s 2>/dev/null", s.K0sFullPath, username, strings.Join(k0sKubeconfigArgs, " "))
	kubeConf, err := ssh.ExecWithOutput(s.Context(), kubeConfigCmd)
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
		return nil, fmt.Errorf("bootloose machine has to have %d port mapped: %w", port, err)
	}
	cfg.Host = fmt.Sprintf("localhost:%d", hostPort)
	return cfg, nil
}

// KubeClient return kube client by loading the admin access config from given node
func (s *BootlooseSuite) KubeClient(node string, k0sKubeconfigArgs ...string) (*kubernetes.Clientset, error) {
	cfg, err := s.GetKubeConfig(node, k0sKubeconfigArgs...)
	if err != nil {
		return nil, err
	}
	return kubernetes.NewForConfig(cfg)
}

// AutopilotClient returns a client for accessing the autopilot schema
func (s *BootlooseSuite) AutopilotClient(node string, k0sKubeconfigArgs ...string) (apclient.Interface, error) {
	cfg, err := s.GetKubeConfig(node, k0sKubeconfigArgs...)
	if err != nil {
		return nil, err
	}
	return apclient.NewForConfig(cfg)
}

// ExtensionsClient returns a client for accessing the extensions schema
func (s *BootlooseSuite) ExtensionsClient(node string, k0sKubeconfigArgs ...string) (*extclient.ApiextensionsV1Client, error) {
	cfg, err := s.GetKubeConfig(node, k0sKubeconfigArgs...)
	if err != nil {
		return nil, err
	}

	return extclient.NewForConfig(cfg)
}

// WaitForNodeReady wait that we see the given node in "Ready" state in kubernetes API
func (s *BootlooseSuite) WaitForNodeReady(name string, kc kubernetes.Interface) error {
	s.T().Logf("waiting to see %s ready in kube API", name)
	if err := WaitForNodeReadyStatus(s.Context(), kc, name, corev1.ConditionTrue); err != nil {
		return err
	}
	s.T().Logf("%s is ready in API", name)
	return nil
}

// GetNodeLabels return the labels of given node
func (s *BootlooseSuite) GetNodeLabels(node string, kc *kubernetes.Clientset) (map[string]string, error) {
	n, err := kc.CoreV1().Nodes().Get(s.Context(), node, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	return n.Labels, nil
}

// WaitForNodeLabel waits for label be assigned to the node
func (s *BootlooseSuite) WaitForNodeLabel(kc *kubernetes.Clientset, node, labelKey, labelValue string) error {
	return watch.Nodes(kc.CoreV1().Nodes()).
		WithObjectName(node).
		WithErrorCallback(RetryWatchErrors(s.T().Logf)).
		Until(s.Context(), func(node *corev1.Node) (bool, error) {
			for k, v := range node.Labels {
				if labelKey == k {
					if labelValue == v {
						return true, nil
					}

					break
				}
			}

			return false, nil
		})
}

// WaitForKubeAPI waits until we see kube API online on given node.
// Timeouts with error return in 5 mins
func (s *BootlooseSuite) WaitForKubeAPI(node string, k0sKubeconfigArgs ...string) error {
	s.T().Logf("waiting for kube api to start on node %s", node)
	return Poll(s.Context(), func(context.Context) (done bool, err error) {
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
		ctx, cancel := context.WithTimeout(s.Context(), 5*time.Second)
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

// WaitJoinApi waits until we see k0s join api up-and-running on a given node.
func (s *BootlooseSuite) WaitJoinAPI(node string) error {
	s.T().Logf("Waiting for k0s join API to start on node %s", node)

	m, err := s.MachineForName(node)
	if err != nil {
		return err
	}
	joinPort, err := m.HostPort(s.K0sAPIExternalPort)
	if err != nil {
		return err
	}
	client := &http.Client{Transport: &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}}
	checkURL := fmt.Sprintf("https://localhost:%d/v1beta1/ca", joinPort)

	return Poll(s.Context(), func(context.Context) (done bool, err error) {
		resp, err := client.Get(checkURL)
		if err != nil {
			return false, nil
		}
		defer resp.Body.Close()

		// JoinAPI returns always un-authorized when called with no token, but it's a signal that it properly up-and-running still
		if resp.StatusCode != http.StatusUnauthorized {
			return false, nil
		}

		s.T().Logf("K0s join API up-and-running")
		return true, nil
	})
}

func (s *BootlooseSuite) initializeBootlooseCluster() error {
	dir, err := os.MkdirTemp("", s.T().Name()+"-bootloose.")
	if err != nil {
		return fmt.Errorf("failed to create temporary directory for bootloose configuration: %w", err)
	}

	err = s.initializeBootlooseClusterInDir(dir)
	if err != nil {
		cleanupClusterDir(s.T(), dir)
	}

	return err
}

// Verifies that kubelet process has the address flag set
func (s *BootlooseSuite) GetKubeletCMDLine(node string) ([]string, error) {
	ssh, err := s.SSH(s.Context(), node)
	if err != nil {
		return nil, err
	}
	defer ssh.Disconnect()

	output, err := ssh.ExecWithOutput(s.Context(), `cat /proc/$(pidof kubelet)/cmdline`)
	if err != nil {
		return nil, err
	}

	return strings.Split(output, "\x00"), nil
}

func (s *BootlooseSuite) initializeBootlooseClusterInDir(dir string) error {
	volumes := []config.Volume{
		{
			Type:        "volume",
			Destination: "/var/lib/k0s",
		},
	}

	volumes, err := s.maybeAddBinPath(volumes)
	if err != nil {
		return err
	}

	if len(s.AirgapImageBundleMountPoints) > 0 {
		airgapPath, ok := os.LookupEnv("K0S_IMAGES_BUNDLE")
		if !ok {
			return errors.New("cannot bind-mount airgap image bundle, environment variable K0S_IMAGES_BUNDLE not set")
		} else if !file.Exists(airgapPath) {
			return fmt.Errorf("cannot bind-mount airgap image bundle, no such file: %q", airgapPath)
		}

		for _, dest := range s.AirgapImageBundleMountPoints {
			volumes = append(volumes, config.Volume{
				Type:        "bind",
				Source:      airgapPath,
				Destination: dest,
				ReadOnly:    true,
			})
		}
	}

	if len(s.K0smotronImageBundleMountPoints) > 0 {
		path, ok := os.LookupEnv("K0SMOTRON_IMAGES_BUNDLE")
		if !ok {
			return errors.New("cannot bind-mount K0smotron image bundle, environment variable K0SMOTRON_IMAGES_BUNDLE not set")
		} else if !file.Exists(path) {
			return fmt.Errorf("cannot bind-mount airgap image bundle, no such file: %q", path)
		}

		for _, dest := range s.K0smotronImageBundleMountPoints {
			volumes = append(volumes, config.Volume{
				Type:        "bind",
				Source:      path,
				Destination: dest,
				ReadOnly:    true,
			})
		}
	}

	// Ensure that kernel config is available in the bootloose boxes.
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
					Image:        s.BootLooseImage,
					Name:         controllerNodeNameFormat,
					Privileged:   true,
					Volumes:      volumes,
					PortMappings: portMaps,
				},
			},
			{
				Count: s.WorkerCount,
				Spec: config.Machine{
					Image:        s.BootLooseImage,
					Name:         workerNodeNameFormat,
					Privileged:   true,
					Volumes:      volumes,
					PortMappings: portMaps,
				},
			},
			{
				Count: s.K0smotronWorkerCount,
				Spec: config.Machine{
					Image:        s.BootLooseImage,
					Name:         k0smotronNodeNameFormat,
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
				Name:         lbNodeNameFormat,
				Image:        defaultBootLooseImage,
				Privileged:   true,
				Volumes:      volumes,
				PortMappings: portMaps,
			},
			Count: 1,
		})
	}

	if s.WithExternalEtcd {
		cfg.Machines = append(cfg.Machines, config.MachineReplicas{
			Spec: config.Machine{
				Name:         etcdNodeNameFormat,
				Image:        defaultBootLooseImage,
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
			},
			Count: 1,
		})
	}

	bootlooseYaml, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("failed to marshal bootloose configuration: %w", err)
	}

	if err = os.WriteFile(path.Join(dir, "bootloose.yaml"), bootlooseYaml, 0700); err != nil {
		return fmt.Errorf("failed to write bootloose configuration to file: %w", err)
	}

	cluster, err := cluster.New(cfg)
	if err != nil {
		return fmt.Errorf("failed to setup a new bootloose cluster: %w", err)
	}

	// we first try to delete instances from previous runs, if they happen to exist
	_ = cluster.Delete()
	if err := cluster.Create(); err != nil {
		return fmt.Errorf("failed to create bootloose cluster: %w", err)
	}

	s.clusterDir = dir
	s.clusterConfig = cfg
	s.cluster = cluster
	return nil
}

func cleanupClusterDir(t *testing.T, dir string) {
	if err := os.RemoveAll(dir); err != nil {
		t.Logf("failed to remove bootloose configuration directory %s: %v", dir, err)
	}
}

func newSuiteContext(t *testing.T) (context.Context, context.CancelCauseFunc) {
	signalCtx, cancel := signalAwareCtx(context.Background())

	// We need to reserve some time to conduct a proper teardown of the suite before the test timeout kicks in.
	deadline, hasDeadline := t.Deadline()
	if !hasDeadline {
		return signalCtx, cancel
	}

	remainingTestDuration := time.Until(deadline)
	//  Let's reserve 10% ...
	reservedTeardownDuration := time.Duration(float64(remainingTestDuration.Milliseconds())*0.10) * time.Millisecond
	// ... but at least 20 seconds.
	reservedTeardownDuration = time.Duration(math.Max(float64(20*time.Second), float64(reservedTeardownDuration)))
	// Then construct the context accordingly.
	deadlineCtx, subCancel := context.WithDeadline(signalCtx, deadline.Add(-reservedTeardownDuration))
	_ = subCancel // Silence linter: the deadlined context is implicitly canceled when canceling the signal context

	return deadlineCtx, cancel
}

// GetControllerIPAddress returns controller ip address
func (s *BootlooseSuite) GetControllerIPAddress(idx int) string {
	return s.getIPAddress(s.ControllerNode(idx))
}

func (s *BootlooseSuite) GetWorkerIPAddress(idx int) string {
	return s.getIPAddress(s.WorkerNode(idx))
}

func (s *BootlooseSuite) GetLBAddress() string {
	return s.getIPAddress(s.LBNode())
}

func (s *BootlooseSuite) GetExternalEtcdIPAddress() string {
	return s.getIPAddress(s.ExternalEtcdNode())
}

func (s *BootlooseSuite) getIPAddress(nodeName string) string {
	ssh, err := s.SSH(s.Context(), nodeName)
	s.Require().NoError(err)
	defer ssh.Disconnect()

	ipAddress, err := ssh.ExecWithOutput(s.Context(), "hostname -i")
	s.Require().NoError(err)
	return ipAddress
}

// RunCommandController runs a command via SSH on a specified controller node
func (s *BootlooseSuite) RunCommandController(idx int, command string) (string, error) {
	ssh, err := s.SSH(s.Context(), s.ControllerNode(idx))
	s.Require().NoError(err)
	defer ssh.Disconnect()

	return ssh.ExecWithOutput(s.Context(), command)
}

// RunCommandWorker runs a command via SSH on a specified controller node
func (s *BootlooseSuite) RunCommandWorker(idx int, command string) (string, error) {
	ssh, err := s.SSH(s.Context(), s.WorkerNode(idx))
	s.Require().NoError(err)
	defer ssh.Disconnect()

	return ssh.ExecWithOutput(s.Context(), command)
}

// GetK0sVersion returns the `k0s version` output from a specific node.
func (s *BootlooseSuite) GetK0sVersion(node string) (string, error) {
	ssh, err := s.SSH(s.Context(), node)
	if err != nil {
		return "", err
	}
	defer ssh.Disconnect()

	version, err := ssh.ExecWithOutput(s.Context(), "/usr/local/bin/k0s version")
	if err != nil {
		return "", err
	}

	return version, nil
}

// GetMembers returns all of the known etcd members for a given node
func (s *BootlooseSuite) GetMembers(idx int) map[string]string {
	// our etcd instances doesn't listen on public IP, so test is performed by calling CLI tools over ssh
	// which in general even makes sense, we can test tooling as well
	sshCon, err := s.SSH(s.Context(), s.ControllerNode(idx))
	s.Require().NoError(err)
	defer sshCon.Disconnect()
	output, err := sshCon.ExecWithOutput(s.Context(), "/usr/local/bin/k0s etcd member-list")
	s.Require().NoError(err)
	output = lastLine(output)

	members := struct {
		Members map[string]string `json:"members"`
	}{}

	s.Require().NoError(json.Unmarshal([]byte(output), &members))

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
func (s *BootlooseSuite) WaitForSSH(node string, timeout time.Duration, delay time.Duration) error {
	s.T().Logf("Waiting for SSH connection to '%s'", node)
	for start := time.Now(); time.Since(start) < timeout; {
		if conn, err := s.SSH(s.Context(), node); err == nil {
			conn.Disconnect()
			return nil
		}

		s.T().Logf("Unable to SSH to '%s', waiting %v for retry", node, delay)
		time.Sleep(delay)
	}

	return fmt.Errorf("timed out waiting for ssh connection to '%s'", node)
}

// GetUpdateServerIPAddress returns the load balancers ip address
func (s *BootlooseSuite) GetUpdateServerIPAddress() string {
	ssh, err := s.SSH(s.Context(), "updateserver0")
	s.Require().NoError(err)
	defer ssh.Disconnect()

	ipAddress, err := ssh.ExecWithOutput(s.Context(), "hostname -i")
	s.Require().NoError(err)
	return ipAddress
}

func (s *BootlooseSuite) AssertSomeKubeSystemPods(client *kubernetes.Clientset) bool {
	if pods, err := client.CoreV1().Pods("kube-system").List(s.Context(), metav1.ListOptions{
		Limit: 100,
	}); s.NoError(err) {
		s.T().Logf("Found %d pods in kube-system", len(pods.Items))
		return s.NotEmpty(pods.Items, "Expected to see some pods in kube-system namespace")
	}

	return false
}

func (s *BootlooseSuite) IsDockerIPv6Enabled() (bool, error) {
	cmd := exec.Command("docker", "inspect", "bridge", "--format", "\"{{ .EnableIPv6 }}\"")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return false, fmt.Errorf("failed to run docker inspect: %w", err)
	}
	var dockerBridgeEnableIPv6 string
	err = json.Unmarshal(output, &dockerBridgeEnableIPv6)
	if err != nil {
		return false, fmt.Errorf("failed to parse default docker bridge EnableIPv6: %w", err)
	}
	bridgeEnableIPv6, err := strconv.ParseBool(dockerBridgeEnableIPv6)
	if err != nil {
		return false, fmt.Errorf("failed to parse default docker bridge EnableIPv6: %w", err)
	}
	return bridgeEnableIPv6, nil
}

func (s *BootlooseSuite) maybeAddBinPath(volumes []config.Volume) ([]config.Volume, error) {
	if os.Getenv("K0S_USE_DEFAULT_K0S_BINARIES") == "true" {
		return volumes, nil
	}

	binPath := os.Getenv("K0S_PATH")
	if binPath == "" {
		return nil, errors.New("failed to locate k0s binary: K0S_PATH environment variable not set")
	}

	fileInfo, err := os.Stat(binPath)
	if err != nil {
		return nil, fmt.Errorf("failed to locate k0s binary %s: %w", binPath, err)
	}
	if fileInfo.IsDir() {
		return nil, fmt.Errorf("failed to locate k0s binary %s: is a directory", binPath)
	}

	updateFromBinPath := os.Getenv("K0S_UPDATE_FROM_PATH")
	if updateFromBinPath != "" {
		volumes = append(volumes, config.Volume{
			Type:        "bind",
			Source:      updateFromBinPath,
			Destination: k0sBindMountFullPath,
			ReadOnly:    true,
		}, config.Volume{
			Type:        "bind",
			Source:      binPath,
			Destination: k0sNewBindMountFullPath,
			ReadOnly:    true,
		})
	} else {
		volumes = append(volumes, config.Volume{
			Type:        "bind",
			Source:      binPath,
			Destination: k0sBindMountFullPath,
			ReadOnly:    true,
		})
	}
	return volumes, nil
}
