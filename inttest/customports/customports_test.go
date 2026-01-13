// SPDX-FileCopyrightText: 2021 k0s authors
// SPDX-License-Identifier: Apache-2.0

package customports

import (
	"bytes"
	"html/template"
	"net"
	"net/url"
	"os"
	"strconv"
	"strings"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8s "k8s.io/client-go/kubernetes"

	"github.com/k0sproject/k0s/inttest/common"
	"github.com/stretchr/testify/suite"
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
	if strings.Contains(os.Getenv("K0S_INTTEST_TARGET"), "dynamicconfig") {
		s.T().Log("Enabling dynamic config for controllers")
		controllerArgs = append(controllerArgs, "--enable-dynamic-config")
	}

	s.Require().NoError(s.InitController(0, controllerArgs...))

	workerToken, err := s.GetJoinToken("worker")
	s.Require().NoError(err)
	s.Require().NoError(s.RunWorkersWithToken(workerToken))

	kc, err := s.KubeClient("controller0")
	s.Require().NoError(err)

	err = s.WaitForNodeReady("worker0", kc)
	s.Require().NoError(err)

	controllerToken, err := s.GetJoinToken("controller")
	s.Require().NoError(err)
	controllerArgs = append([]string{controllerToken, ""}, controllerArgs...)
	s.Require().NoError(s.InitController(1, controllerArgs...))
	s.Require().NoError(s.InitController(2, controllerArgs...))

	s.Require().NoError(err)

	s.AssertSomeKubeSystemPods(kc)

	s.T().Log("waiting to see CNI pods ready")
	s.Require().NoError(common.WaitForKubeRouterReady(s.Context(), kc), "kube-router did not start")
	s.T().Log("waiting to see konnectivity-agent pods ready")
	s.Require().NoError(common.WaitForDaemonSet(s.Context(), kc, "konnectivity-agent", metav1.NamespaceSystem), "konnectivity-agent did not start")

	s.T().Log("waiting to get logs from pods")
	s.Require().NoError(common.WaitForPodLogs(s.Context(), kc, metav1.NamespaceSystem))

	// https://github.com/k0sproject/k0s/issues/1202
	s.Run("kubeconfigIncludesExternalAddress", func() {
		expectedURL := url.URL{Scheme: "https", Host: net.JoinHostPort(ipAddress, strconv.Itoa(kubeAPIPort))}
		ssh, err := s.SSH(s.Context(), s.ControllerNode(0))
		s.Require().NoError(err)
		defer ssh.Disconnect()

		out, err := ssh.ExecWithOutput(s.Context(), "/usr/local/bin/k0s kubeconfig create user | awk '$1 == \"server:\" {print $2}'")
		s.Require().NoError(err)
		s.Require().Equal(expectedURL.String(), out)
	})
}
