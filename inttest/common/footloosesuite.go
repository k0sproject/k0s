package common

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"io/ioutil"
	"os"
	"os/signal"
	"path"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/Mirantis/mke/pkg/util"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/suite"
	"gopkg.in/yaml.v2"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/weaveworks/footloose/pkg/cluster"
	"github.com/weaveworks/footloose/pkg/config"

	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
)

// FootlooseSuite defines all the common stuff we need to be able to run mke testing on footloose
type FootlooseSuite struct {
	suite.Suite

	Cluster *cluster.Cluster

	ControllerCount int
	WorkerCount     int

	tearDownTimer *time.Timer

	footlooseConfig config.Config

	keyDir string
}

// SetupSuite does all the setup work, namely boots up footloose cluster
func (s *FootlooseSuite) SetupSuite() {
	dir, err := ioutil.TempDir("", "footloose-keys")
	if err != nil {
		s.T().Logf("ERROR: failed to load footloose config: %s", err.Error())
		s.T().FailNow()
	}
	s.keyDir = dir
	s.footlooseConfig = s.createConfig()
	cluster, err := cluster.New(s.footlooseConfig)
	if err != nil {
		s.T().Logf("ERROR: failed to load footloose config: %s", err.Error())
		s.T().FailNow()
		return
	}

	_ = cluster.Delete()

	err = cluster.Create()
	if err != nil {
		s.FailNowf("failed to create footloose cluster: %s", err.Error())
		s.T().FailNow()
		return
	}
	s.Cluster = cluster

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

	// SSH through cluster will wait until we actually can get it through
	err = s.Cluster.SSH("controller0", "root", "hostname")
	if err != nil {
		s.FailNowf("failed to ssh to controller0: %s", err.Error())
		s.T().FailNow()
		return
	}
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
		ssh, err := s.SSH(m.Hostname())
		if err != nil {
			s.T().Logf("failed to ssh to node %s to get logs", m.Hostname())
			continue
		}
		log, err := ssh.ExecWithOutput("cat /tmp/mke-*.log")
		logPath := path.Join("/tmp", fmt.Sprintf("%s.log", m.Hostname()))
		err = ioutil.WriteFile(logPath, []byte(log), 0700)
		s.T().Logf("wrote log of node %s to %s", m.Hostname(), logPath)
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

const keepAfterTestsEnv = "MKE_KEEP_AFTER_TESTS"

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

// RunControllers runs all the controller nodes
// TODO Supports currently only one controller, we need to automate running and joining
func (s *FootlooseSuite) RunControllers() error {
	if s.ControllerCount > 1 {
		return fmt.Errorf("RunControllers does not yet support many controllers")
	}
	controllerNode := fmt.Sprintf("controller%d", 0)
	ssh, err := s.SSH(controllerNode)
	if err != nil {
		return err
	}
	defer ssh.Disconnect()

	_, err = ssh.ExecWithOutput("nohup mke server >/tmp/mke-server.log 2>&1 &")
	if err != nil {
		return err
	}
	return s.WaitForKubeAPI("controller0")
}

// RunWorkers joins all the workers to the cluster
func (s *FootlooseSuite) RunWorkers() error {
	ssh, err := s.SSH("controller0")
	if err != nil {
		return err
	}
	defer ssh.Disconnect()

	token, err := ssh.ExecWithOutput("mke token create --role=worker")
	if err != nil {
		return err
	}
	if token == "" {
		return fmt.Errorf("got empty token for worker join")
	}

	workerCommand := fmt.Sprintf(`nohup mke worker "%s" >/tmp/mke-worker.log 2>&1 &`, token)
	for i := 0; i < s.WorkerCount; i++ {
		workerNode := fmt.Sprintf("worker%d", i)
		sshWorker, err := s.SSH(workerNode)
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

// KubeClient return kube client by loading the admin access config from given node
func (s *FootlooseSuite) KubeClient(node string) (*kubernetes.Clientset, error) {
	machine, err := s.MachineForName(node)
	if err != nil {
		return nil, err
	}
	ssh, err := s.SSH(node)
	if err != nil {
		return nil, err
	}
	kubeConf, err := ssh.ExecWithOutput("cat /var/lib/mke/pki/admin.conf")
	if err != nil {
		return nil, err
	}
	cfg, err := clientcmd.RESTConfigFromKubeConfig([]byte(kubeConf))
	if err != nil {
		return nil, err
	}
	hostPort, err := machine.HostPort(6443)
	if err != nil {
		return nil, errors.Wrap(err, "footloose machine has to have 6443 port mapped")
	}
	cfg.Host = fmt.Sprintf("localhost:%d", hostPort)
	return kubernetes.NewForConfig(cfg)
}

// WaitForNodeReady wait that we see the given node in "Ready" state in kubernetes API
func (s *FootlooseSuite) WaitForNodeReady(node string, kc *kubernetes.Clientset) error {
	s.T().Logf("waiting to see %s ready in kube API", node)
	return wait.PollImmediate(1*time.Second, 5*time.Minute, func() (done bool, err error) {
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

// WaitForKubeAPI waits until we see kube API online on given node.
// Timeouts with error return in 5 mins
func (s *FootlooseSuite) WaitForKubeAPI(node string) error {
	s.T().Log("starting to poll kube api")
	return wait.PollImmediate(1*time.Second, 5*time.Minute, func() (done bool, err error) {
		kc, err := s.KubeClient(node)
		if err != nil {
			return false, nil
		}
		v, err := kc.ServerVersion()
		if err != nil {
			return false, nil
		}
		s.T().Logf("kube api seems to be up-and-running, version: %s", v.String())
		return true, nil
	})
}

func (s *FootlooseSuite) createConfig() config.Config {
	mkePath := os.Getenv("MKE_PATH")
	if mkePath == "" {
		s.FailNow("MKE_PATH env needs to be set to MKE binary")
	}

	volumes := []config.Volume{
		config.Volume{
			Type:        "bind",
			Source:      "/lib/modules",
			Destination: "/lib/modules",
		},
		config.Volume{
			Type:        "bind",
			Source:      mkePath,
			Destination: "/usr/bin/mke",
		},
		config.Volume{
			Type:        "volume",
			Destination: "/var/lib/mke",
		},
	}

	portMaps := []config.PortMapping{
		config.PortMapping{
			ContainerPort: 22,
		},
		config.PortMapping{
			ContainerPort: 6443,
		},
	}

	return config.Config{
		Cluster: config.Cluster{
			Name:       s.T().Name(),
			PrivateKey: path.Join(s.keyDir, "id_rsa"),
		},
		Machines: []config.MachineReplicas{
			config.MachineReplicas{
				Count: s.ControllerCount,
				Spec: config.Machine{
					Image:        "quay.io/footloose/ubuntu18.04",
					Name:         "controller%d",
					Privileged:   true,
					Volumes:      volumes,
					PortMappings: portMaps,
				},
			},
			config.MachineReplicas{
				Count: s.WorkerCount,
				Spec: config.Machine{
					Image:        "quay.io/footloose/ubuntu18.04",
					Name:         "worker%d",
					Privileged:   true,
					Volumes:      volumes,
					PortMappings: portMaps,
				},
			},
		},
	}

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
			// Let's shave 10% off, so mke suite has enough time to run teardown
			testTimeout := time.Duration(float64(timeout.Milliseconds())*0.90) * time.Millisecond
			return testTimeout
		}
	}
	return DefaultTimeout
}

func generateKeyPair(dir string) error {
	keyFile := filepath.Join(dir, "id_rsa.key")
	pubFile := filepath.Join(dir, "id_rsa.pub")

	reader := rand.Reader
	bitSize := 2048

	key, err := rsa.GenerateKey(reader, bitSize)
	if err != nil {
		return err
	}

	var privateKey = &pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(key),
	}

	outFile, err := os.Create(keyFile)
	if err != nil {
		return err
	}
	defer outFile.Close()

	err = pem.Encode(outFile, privateKey)

	// note to the next reader: key.Public() != key.PublicKey
	pubBytes, err := x509.MarshalPKIXPublicKey(key.Public())
	if err != nil {
		return err
	}

	var pemkey = &pem.Block{
		Type:  "PUBLIC KEY",
		Bytes: pubBytes,
	}

	pemfile, err := os.Create(pubFile)
	if err != nil {
		return err
	}
	defer pemfile.Close()

	err = pem.Encode(pemfile, pemkey)
	if err != nil {
		return err
	}

	return nil
}
