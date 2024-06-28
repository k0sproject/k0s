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

package customports

import (
	"bytes"
	"fmt"
	"html/template"
	"os"
	"testing"

	"github.com/k0sproject/k0s/inttest/common"
	"github.com/stretchr/testify/suite"
	k8s "k8s.io/client-go/kubernetes"
)

type customPortsSuite struct {
	common.BootlooseSuite

	client *k8s.Clientset
}

const configWithExternaladdress = `
apiVersion: k0s.k0sproject.io/v1beta1
kind: ClusterConfig
metadata:
  name: k0s
spec:
  api:
    externalAddress: {{ .Address }}
    port: {{ .KubePort }}
    k0sApiPort: {{ .K0sPort }}
    sans:
      - {{ .Address }}
  konnectivity:
    agentPort: {{ .KonnectivityAgentPort }}
    adminPort: {{ .KonnectivityAdminPort }}
`

const kubeAPIPort = 7443
const k0sAPIPort = 9743
const agentPort = 9132
const adminPort = 9133

func TestCustomPorts(t *testing.T) {

	s := customPortsSuite{
		common.BootlooseSuite{
			ControllerCount:       3,
			WorkerCount:           1,
			KubeAPIExternalPort:   kubeAPIPort,
			K0sAPIExternalPort:    k0sAPIPort,
			KonnectivityAgentPort: agentPort,
			KonnectivityAdminPort: adminPort,
			WithLB:                true,
		},
		nil,
	}
	suite.Run(t, &s)
}

func (s *customPortsSuite) getControllerConfig(ipAddress string) string {
	data := struct {
		Address               string
		KubePort              int
		K0sPort               int
		KonnectivityAgentPort int
		KonnectivityAdminPort int
	}{
		Address:               ipAddress,
		KubePort:              kubeAPIPort,
		K0sPort:               k0sAPIPort,
		KonnectivityAgentPort: agentPort,
		KonnectivityAdminPort: adminPort,
	}
	content := bytes.NewBuffer([]byte{})
	s.Require().NoError(template.Must(template.New("k0s.yaml").Parse(configWithExternaladdress)).Execute(content, data), "can't execute k0s.yaml template")
	return content.String()
}

func (s *customPortsSuite) TestControllerJoinsWithCustomPort() {
	ipAddress := s.GetLBAddress()
	s.T().Logf("ip address: %s", ipAddress)
	config := s.getControllerConfig(ipAddress)
	s.PutFile("controller0", "/tmp/k0s.yaml", config)
	s.PutFile("controller1", "/tmp/k0s.yaml", config)
	s.PutFile("controller2", "/tmp/k0s.yaml", config)

	controllerArgs := []string{"--config=/tmp/k0s.yaml"}
	if os.Getenv("K0S_ENABLE_DYNAMIC_CONFIG") == "true" {
		s.T().Log("Enabling dynamic config for controllers")
		controllerArgs = append(controllerArgs, "--enable-dynamic-config")
	}

	s.Require().NoError(s.InitController(0, controllerArgs...))

	workerToken, err := s.GetJoinToken("worker", "")
	s.Require().NoError(err)
	s.Require().NoError(s.RunWorkersWithToken("/var/lib/k0s", workerToken))

	kc, err := s.KubeClient("controller0", "")
	s.Require().NoError(err)

	err = s.WaitForNodeReady("worker0", kc)
	s.Require().NoError(err)

	controllerToken, err := s.GetJoinToken("controller", "")
	s.Require().NoError(err)
	controllerArgs = append([]string{controllerToken, ""}, controllerArgs...)
	s.Require().NoError(s.InitController(1, controllerArgs...))
	s.Require().NoError(s.InitController(2, controllerArgs...))

	s.Require().NoError(err)

	s.AssertSomeKubeSystemPods(kc)

	s.T().Log("waiting to see CNI pods ready")
	s.Require().NoError(common.WaitForKubeRouterReady(s.Context(), kc), "calico did not start")
	s.T().Log("waiting to see konnectivity-agent pods ready")
	s.Require().NoError(common.WaitForDaemonSet(s.Context(), kc, "konnectivity-agent", "kube-system"), "konnectivity-agent did not start")

	s.T().Log("waiting to get logs from pods")
	s.Require().NoError(common.WaitForPodLogs(s.Context(), kc, "kube-system"))

	// https://github.com/k0sproject/k0s/issues/1202
	s.Run("kubeconfigIncludesExternalAddress", func() {
		ssh, err := s.SSH(s.Context(), s.ControllerNode(0))
		s.Require().NoError(err)
		defer ssh.Disconnect()

		out, err := ssh.ExecWithOutput(s.Context(), "/usr/local/bin/k0s kubeconfig create user | awk '$1 == \"server:\" {print $2}'")
		s.Require().NoError(err)
		s.Require().Equal(fmt.Sprintf("https://%s:%d", ipAddress, kubeAPIPort), out)
	})
}
