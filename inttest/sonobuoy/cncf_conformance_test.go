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
package sonobuoy

import (
	"fmt"
	"os"
	"os/exec"
	"path"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"

	"github.com/avast/retry-go"

	"github.com/k0sproject/k0s/inttest/common"
	"k8s.io/client-go/tools/clientcmd"
)

type ConformanceSuite struct {
	common.VMSuite
	sonoBin string
}

func (s *ConformanceSuite) TestConformance() {
	s.NoError(s.InitMainController())
	s.NoError(s.RunWorkers())

	kc, err := s.KubeClient(s.ControllerIP)
	s.NoError(err)

	kubeConfig, err := s.GetKubeConfig(s.ControllerIP)
	s.NoError(err)

	err = s.WaitForNodeReady("worker-0", kc)
	s.NoError(err)

	err = s.WaitForNodeReady("worker-1", kc)
	s.NoError(err)

	s.T().Log("waiting to see metrics ready")
	s.Require().NoError(common.WaitForMetricsReady(kubeConfig))

	kubeconfigPath := s.dumpKubeConfig()
	s.T().Logf("kubeconfig at: %s", kubeconfigPath)

	err = os.Setenv("KUBECONFIG", kubeconfigPath)
	s.NoError(err)

	sonoArgs := []string{
		"run",
		"--mode=certified-conformance",
		"--wait=1200",
		fmt.Sprintf("--kubernetes-version=%s", kubeConformanceImageVersion),
	}
	s.T().Log("running sonobuoy conformance testing, this WILL a looong while")
	sonoFinished := make(chan bool)
	go func() {
		timer := time.NewTicker(1 * time.Minute)
		defer timer.Stop()
		for {
			select {
			case <-sonoFinished:
				return
			case <-timer.C:
				s.T().Logf("sonobuoy still running, please wait...")
				s.getRunningSonobuoyStatus()
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

	s.T().Log("sonobuoy has been ran successfully, collecting results")
	results, err := s.retrieveResults()
	s.NoError(err)
	s.T().Logf("sonobuoy results:%+v", results)

	s.Equal("passed", results.Status)
	s.Equal(0, results.Failed)
	if results.Status != "passed" {
		s.T().Logf("sonobuoy run failed. will attempt to re-run failed tests")
		err = s.reRunFailedTests(results.ResultPath)
	}
	s.NoError(err)
}

func (s *ConformanceSuite) dumpKubeConfig() string {
	dir, err := os.MkdirTemp("", "conformance-kubeconfig-")
	s.NoError(err)
	ssh, err := s.SSH(s.ControllerIP)
	s.NoError(err)
	defer ssh.Disconnect()

	kubeConf, err := ssh.ExecWithOutput("sudo -h 127.0.0.1 cat /var/lib/k0s/pki/admin.conf")
	s.NoError(err)

	cfg, err := clientcmd.Load([]byte(kubeConf))
	s.NoError(err)

	cfg.Clusters["local"].Server = fmt.Sprintf("https://%s:%d", s.ControllerIP, 6443)
	// Our CA data is valid for localhost, but we need to change that in order to connect from outside
	cfg.Clusters["local"].InsecureSkipTLSVerify = true
	cfg.Clusters["local"].CertificateAuthorityData = nil

	kubeconfigPath := path.Join(dir, "kubeconfig")
	err = clientcmd.WriteToFile(*cfg, kubeconfigPath)
	s.NoError(err)

	return kubeconfigPath
}

func (s *ConformanceSuite) retrieveResults() (Result, error) {
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

func (s *ConformanceSuite) reRunFailedTests(resultsPath string) error {
	s.T().Log("re-running failed sonobuoy tests")
	kubeconfigPath := s.dumpKubeConfig()

	err := os.Setenv("KUBECONFIG", kubeconfigPath)
	s.NoError(err)

	sonoArgs := []string{
		"e2e",
		resultsPath,
		"--wait=1200",
		"--rerun-failed",
		fmt.Sprintf("--kubernetes-version=%s", kubeConformanceImageVersion),
	}

	sonoFinished := make(chan bool)
	go func() {
		timer := time.NewTicker(1 * time.Minute)
		defer timer.Stop()
		for {
			select {
			case <-sonoFinished:
				return
			case <-timer.C:
				s.T().Logf("sonobuoy still running, please wait...")
				s.getRunningSonobuoyStatus()
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

	s.T().Log("sonobuoy has been ran successfully, collecting results")
	results, err := s.retrieveResults()
	s.NoError(err)
	s.T().Logf("sonobuoy results:%+v", results)

	s.Equal("passed", results.Status)
	s.Equal(0, results.Failed)
	if results.Status != "passed" {
		s.T().Logf("sonobuoy run failed, you can see more details on the failing tests with: %s results %s", s.sonoBin, results.ResultPath)
	}
	return err
}

func TestConformanceSuite(t *testing.T) {
	sonoPath := os.Getenv("SONOBUOY_PATH")
	if sonoPath == "" {
		t.Fatal("SONOBUOY_PATH env needs to be set")
	}
	s := ConformanceSuite{
		common.VMSuite{
			KeyDir: "../conformance/terraform",
		},
		sonoPath,
	}
	suite.Run(t, &s)
}

func (s *ConformanceSuite) getRunningSonobuoyStatus() {
	statusCmd := exec.Command(s.sonoBin, "status")
	statusCmd.Stdout = os.Stdout
	statusCmd.Stderr = os.Stderr
	err := statusCmd.Run()
	if err != nil {
		s.T().Logf("error fetching sonobouy status: %s", err.Error())
	}
}
