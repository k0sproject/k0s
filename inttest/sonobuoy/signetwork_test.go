package sonobuoy

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path"
	"strings"
	"testing"
	"time"

	"github.com/Mirantis/mke/inttest/common"
	"github.com/avast/retry-go"
	"github.com/stretchr/testify/suite"
	"k8s.io/client-go/tools/clientcmd"
)

type NetworkSuite struct {
	common.FootlooseSuite
	sonoBin string
}

func (s *NetworkSuite) TestSigNetwork() {
	s.NoError(s.InitMainController())
	s.NoError(s.RunWorkers())

	kc, err := s.KubeClient("controller0")
	s.NoError(err)

	err = s.WaitForNodeReady("worker0", kc)
	s.NoError(err)

	err = s.WaitForNodeReady("worker1", kc)
	s.NoError(err)

	kubeconfigPath := s.dumpKubeConfig()
	s.T().Logf("kubeconfig at: %s", kubeconfigPath)

	err = os.Setenv("KUBECONFIG", kubeconfigPath)
	s.NoError(err)

	sonoArgs := []string{
		"run",
		"--wait=1200", // 20mins
		"--plugin-env=e2e.E2E_USE_GO_RUNNER=true",
		`--e2e-focus=\[sig-network\].*\[Conformance\]`,
		`--e2e-skip=\[Serial\]`,
		"--e2e-parallel=y",
	}
	s.T().Log("running sonobuoy, this may take a while")
	sonoFinished := make(chan bool)
	go func() {
		timer := time.NewTicker(30 * time.Second)
		defer timer.Stop()
		for {
			select {
			case <-sonoFinished:
				return
			case <-timer.C:
				s.T().Logf("sonobuoy still running, please wait...")
			}
		}
	}()
	sonoCmd := exec.Command(s.sonoBin, sonoArgs...)
	sonoCmd.Stdout = os.Stdout
	sonoCmd.Stderr = os.Stderr
	err = sonoCmd.Run()
	sonoFinished <- true
	if err != nil {
		s.T().Logf("error executing sonobouy: %s", err.Error())
	}
	s.NoError(err)

	s.T().Log("sonobuoy has been ran succesfully, collecting results")
	results, err := s.retrieveResults()
	s.NoError(err)
	s.T().Logf("sonobuoy results:%+v", results)

	s.Equal("passed", results.Status)
	s.Equal(0, results.Failed)
	if results.Status != "passed" {
		s.T().Logf("sonobuoy run failed, you can see more details on the failing tests with: %s results %s", s.sonoBin, results.ResultPath)
	}

}

func (s *NetworkSuite) retrieveResults() (Result, error) {

	var resultPath string

	err := retry.Do(func() error {
		retrieveCmd := exec.Command(s.sonoBin, "retrieve")
		retrieveOutput, err := retrieveCmd.Output()
		if err != nil {
			return err
		}

		resultPath = strings.Trim(string(retrieveOutput), "\n")
		return nil
	}, retry.Attempts(3))

	if err != nil {
		return Result{}, err
	}
	cwd, err := os.Getwd()
	if err != nil {
		return Result{}, err
	}
	resultPath = path.Join(cwd, resultPath)

	s.T().Logf("sonobuoy results stored at: %s", resultPath)

	resultArgs := []string{
		"results",
		"--plugin=e2e",
		resultPath,
	}
	resultCmd := exec.Command(s.sonoBin, resultArgs...)
	resultOutput, err := resultCmd.CombinedOutput()
	if err != nil {
		s.T().Logf("sono results output:\n%s", string(resultOutput))
		return Result{}, err
	}
	result, err := ResultFromString(string(resultOutput))
	result.ResultPath = resultPath
	return result, err

}

func (s *NetworkSuite) dumpKubeConfig() string {
	machine, err := s.MachineForName("controller0")
	s.NoError(err)
	hostPort, err := machine.HostPort(6443)
	s.NoError(err)

	dir, err := ioutil.TempDir("", "sig-network-kubeconfig-")
	s.NoError(err)
	ssh, err := s.SSH("controller0")
	s.NoError(err)
	defer ssh.Disconnect()

	kubeConf, err := ssh.ExecWithOutput("cat /var/lib/mke/pki/admin.conf")
	s.NoError(err)

	cfg, err := clientcmd.Load([]byte(kubeConf))
	s.NoError(err)

	cfg.Clusters["local"].Server = fmt.Sprintf("https://localhost:%d", hostPort)

	kubeconfigPath := path.Join(dir, "kubeconfig")

	err = clientcmd.WriteToFile(*cfg, kubeconfigPath)
	s.NoError(err)

	return kubeconfigPath
}

func TestNetworkSuite(t *testing.T) {
	sonoPath := os.Getenv("SONOBUOY_PATH")
	if sonoPath == "" {
		t.Fatal("SONOBUOY_PATH env needs to be set")
	}
	s := NetworkSuite{
		common.FootlooseSuite{
			ControllerCount: 1,
			WorkerCount:     2,
		},
		sonoPath,
	}

	suite.Run(t, &s)
}
