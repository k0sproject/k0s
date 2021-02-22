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
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path"
	"strings"
	"time"

	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"k8s.io/apimachinery/pkg/util/wait"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/stretchr/testify/suite"
)

// TerraformMachineData is the Golang representation of the terraform output
type TerraformMachineData struct {
	Controllers struct {
		IP []string `json:"value"`
	} `json:"controller_external_ip"`
	Workers struct {
		IP []string `json:"value"`
	} `json:"worker_external_ip"`
}

// VMSuite
type VMSuite struct {
	suite.Suite
	config       TerraformMachineData
	ControllerIP string
	WorkerIPs    []string
	keyDir       string
}

func (s *VMSuite) GetConfig() {
	tfDataFile, err := os.Open("../terraform/test-cluster/out.json")
	if err != nil {
		s.T().Logf("failed to read terraform output: %s", err.Error())
	}
	defer tfDataFile.Close()

	var tfMachineData TerraformMachineData

	jsonParser := json.NewDecoder(tfDataFile)
	if err = jsonParser.Decode(&tfMachineData); err != nil {
		s.T().Logf("failed to marshall terraform json: %s", err.Error())
	}

	// set our VMSuite configuration
	s.config = tfMachineData
	s.ControllerIP = tfMachineData.Controllers.IP[0]
	s.keyDir = "../terraform/test-cluster"
	s.WorkerIPs = tfMachineData.Workers.IP
}

// InitMainController inits first controller assuming it's first controller in the cluster
func (s *VMSuite) InitMainController() error {
	s.GetConfig()
	controllerNode := s.ControllerIP
	ssh, err := s.SSH(controllerNode)
	if err != nil {
		return err
	}
	defer ssh.Disconnect()

	startControllerCmd := "sudo nohup k0s controller --debug >/tmp/k0s-controller.log 2>&1 &"
	_, err = ssh.ExecWithOutput(startControllerCmd)
	if err != nil {
		return err
	}
	return s.WaitForKubeAPI(controllerNode)
}

// WaitForKubeAPI waits until we see kube API online on given node.
// Timeouts with error return in 5 mins
func (s *VMSuite) WaitForKubeAPI(node string) error {
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

// SSH establishes an SSH connection to the node
func (s *VMSuite) SSH(ip string) (*SSHConnection, error) {
	ssh := &SSHConnection{
		Address: ip,
		User:    "ubuntu",
		Port:    22,
		KeyPath: path.Join(s.keyDir, "aws_private.pem"),
	}

	err := ssh.Connect()
	if err != nil {
		return nil, err
	}

	return ssh, nil
}

// RunWorkers joins all the workers to the cluster
func (s *VMSuite) RunWorkers() error {
	ssh, err := s.SSH(s.ControllerIP)
	if err != nil {
		return err
	}
	defer ssh.Disconnect()
	token, err := s.GetJoinToken("worker")
	if err != nil {
		return err
	}
	if token == "" {
		return fmt.Errorf("got empty token for worker join")
	}
	workerCommand := fmt.Sprintf(`sudo nohup k0s worker --debug "%s" >/tmp/k0s-worker.log 2>&1 &`, token)
	for i := 0; i < len(s.WorkerIPs); i++ {
		workerNode := s.WorkerIPs[i]
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

// GetJoinToken generates join token for the asked role
func (s *VMSuite) GetJoinToken(role string) (string, error) {
	// assume we have main on 1 node always
	s.Contains([]string{"controller", "worker"}, role, "Bad role")
	ssh, err := s.SSH(s.ControllerIP)
	if err != nil {
		return "", err
	}
	defer ssh.Disconnect()
	tokenCmd := fmt.Sprintf("sudo -h 127.0.0.1 k0s token create --role=%s", role)
	token, err := ssh.ExecWithOutput(tokenCmd)
	if err != nil {
		return "", fmt.Errorf("can't get join token: %v", err)
	}
	outputParts := strings.Split(token, "\n")
	// in case of no k0s.conf given, there might be warnings on the first few lines
	token = outputParts[len(outputParts)-1]
	return token, nil

}

// WaitForNodeReady wait that we see the given node in "Ready" state in kubernetes API
func (s *VMSuite) WaitForNodeReady(node string, kc *kubernetes.Clientset) error {
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

// KubeClient return kube client by loading the admin access config from given node
func (s *VMSuite) KubeClient(node string) (*kubernetes.Clientset, error) {
	ssh, err := s.SSH(node)
	if err != nil {
		return nil, err
	}
	// sudo -h 127.0.0.1 is used to override `unable to resolve host XXX` errors when running sudo
	kubeConf, err := ssh.ExecWithOutput("sudo -h 127.0.0.1 cat /var/lib/k0s/pki/admin.conf")
	if err != nil {
		return nil, err
	}
	cfg, err := clientcmd.RESTConfigFromKubeConfig([]byte(kubeConf))
	if err != nil {
		return nil, err
	}

	// Configure k8s Client
	cfg.Host = fmt.Sprintf("%s:%d", node, 6443)

	// Our CA data is valid for localhost, but we need to change that in order to connect from outside
	cfg.Insecure = true
	cfg.TLSClientConfig.CAData = nil

	return kubernetes.NewForConfig(cfg)
}
