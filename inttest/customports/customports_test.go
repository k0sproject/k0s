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
	"context"
	"html/template"
	"testing"

	"github.com/k0sproject/k0s/inttest/common"
	"github.com/stretchr/testify/suite"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8s "k8s.io/client-go/kubernetes"
)

type Suite struct {
	common.FootlooseSuite

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

const (
	kubeAPIPort = 7443
	k0sAPIPort  = 9743
	agentPort   = 9132
	adminPort   = 9133
)

func TestSuite(t *testing.T) {
	s := Suite{
		common.FootlooseSuite{
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

func (ds *Suite) getControllerConfig(ipAddress string) string {
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
	ds.Require().NoError(template.Must(template.New("k0s.yaml").Parse(configWithExternaladdress)).Execute(content, data), "can't execute k0s.yaml template")
	return content.String()
}

func (ds *Suite) TestControllerJoinsWithCustomPort() {
	ipAddress := ds.GetControllerIPAddress(0)
	ds.T().Logf("ip address: %s", ipAddress)
	config := ds.getControllerConfig(ipAddress)
	ds.PutFile("controller0", "/tmp/k0s.yaml", config)
	ds.PutFile("controller1", "/tmp/k0s.yaml", config)
	ds.PutFile("controller2", "/tmp/k0s.yaml", config)

	ds.Require().NoError(ds.InitController(0, "--config=/tmp/k0s.yaml"))

	token, err := ds.GetJoinToken("controller", "", "--config=/tmp/k0s.yaml")
	ds.Require().NoError(err)
	ds.Require().NoError(ds.InitController(1, token, "", "--config=/tmp/k0s.yaml"))

	ds.Require().NoError(ds.InitController(2, token, "", "--config=/tmp/k0s.yaml"))

	token, err = ds.GetJoinToken("worker", "", "--config=/tmp/k0s.yaml")
	ds.Require().NoError(err)
	ds.Require().NoError(ds.RunWorkersWithToken("/var/lib/k0s", token, `--config="/tmp/k0s.yaml"`))

	kc, err := ds.KubeClient("controller0", "")
	ds.Require().NoError(err)

	err = ds.WaitForNodeReady("worker0", kc)

	ds.Require().NoError(err)

	pods, err := kc.CoreV1().Pods("kube-system").List(context.TODO(), v1.ListOptions{
		Limit: 100,
	})
	ds.Require().NoError(err)

	podCount := len(pods.Items)
	ds.T().Logf("found %d pods in kube-system", podCount)
	ds.Greater(podCount, 0, "expecting to see few pods in kube-system namespace")

	ds.T().Log("waiting to see calico pods ready")
	ds.Require().NoError(common.WaitForKubeRouterReady(kc), "calico did not start")
	ds.T().Log("waiting to see konnectivity-agent pods ready")
	ds.Require().NoError(common.WaitForDaemonSet(kc, "konnectivity-agent"), "konnectivity-agent did not start")

	ds.T().Log("waiting to get logs from pods")
	ds.Require().NoError(common.WaitForPodLogs(kc, "kube-system"))
}
