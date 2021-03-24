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
	"context"
	"fmt"
	"github.com/k0sproject/k0s/inttest/common"
	"github.com/stretchr/testify/suite"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8s "k8s.io/client-go/kubernetes"
	"testing"
)

type Suite struct {
	common.FootlooseSuite

	client *k8s.Clientset
}

const configWithExternaladdress = `
apiVersion: k0s.k0sproject.io/v1beta1
kind: Cluster
metadata:
  name: k0s
spec:
  api:
    externalAddress: %s
    port: %d
    k0s_api_port: %d
  konnectivity:
    agent_port: %d
    admin_port: %d
`

const APIPort = 7443
const k0sAPIPort = 9743
const agentPort = 9132
const adminPort = 9133

func TestSuite(t *testing.T) {

	s := Suite{
		common.FootlooseSuite{
			ControllerCount:     3,
			WorkerCount:         1,
			KubeAPIExternalPort: APIPort,
			K0sAPIExternalPort:  k0sAPIPort,
		},
		nil,
	}
	suite.Run(t, &s)
}

func (ds *Suite) getMainIPAddress() string {
	ssh, err := ds.SSH("controller0")
	ds.Require().NoError(err)
	defer ssh.Disconnect()

	ipAddress, err := ssh.ExecWithOutput("hostname -i")
	ds.Require().NoError(err)
	return ipAddress
}

func (ds *Suite) putFile(node string, path string, content string) {
	ssh, err := ds.SSH(node)
	ds.Require().NoError(err)
	defer ssh.Disconnect()
	_, err = ssh.ExecWithOutput(fmt.Sprintf("echo '%s' >%s", content, path))

	ds.Require().NoError(err)

}

func (ds *Suite) TestControllerJoinsWithCustomPort() {

	ipAddress := ds.getMainIPAddress()
	ds.T().Logf("ip address: %s", ipAddress)

	ds.putFile("controller0", "/tmp/k0s.yaml", fmt.Sprintf(configWithExternaladdress, ipAddress, APIPort, k0sAPIPort, agentPort, adminPort))
	ds.putFile("controller1", "/tmp/k0s.yaml", fmt.Sprintf(configWithExternaladdress, ipAddress, APIPort, k0sAPIPort, agentPort, adminPort))
	ds.putFile("controller2", "/tmp/k0s.yaml", fmt.Sprintf(configWithExternaladdress, ipAddress, APIPort, k0sAPIPort, agentPort, adminPort))
	ds.putFile("worker0", "/tmp/k0s.yaml", fmt.Sprintf(configWithExternaladdress, ipAddress, APIPort, k0sAPIPort, agentPort, adminPort))
	ds.NoError(ds.InitController(0, "--config=/tmp/k0s.yaml"))

	token, err := ds.GetJoinToken("controller", "", "--config=/tmp/k0s.yaml")
	ds.NoError(err)
	ds.putFile("controller1", "/tmp/k0s.yaml", fmt.Sprintf(configWithExternaladdress, ipAddress, APIPort, k0sAPIPort, agentPort, adminPort))
	ds.NoError(ds.InitController(1, token, "", "--config=/tmp/k0s.yaml"))

	ds.putFile("controller2", "/tmp/k0s.yaml", fmt.Sprintf(configWithExternaladdress, ipAddress, APIPort, k0sAPIPort, agentPort, adminPort))
	ds.NoError(ds.InitController(2, token, "", "--config=/tmp/k0s.yaml"))

	token, err = ds.GetJoinToken("worker", "", "--config=/tmp/k0s.yaml")
	ds.NoError(err)
	ds.NoError(ds.RunWorkersWithToken("/var/lib/k0s", token, `--config="/tmp/k0s.yaml"`))

	kc, err := ds.KubeClient("controller0", "")
	ds.NoError(err)

	err = ds.WaitForNodeReady("worker0", kc)

	ds.NoError(err)

	pods, err := kc.CoreV1().Pods("kube-system").List(context.TODO(), v1.ListOptions{
		Limit: 100,
	})
	ds.NoError(err)

	podCount := len(pods.Items)
	//
	ds.T().Logf("found %d pods in kube-system", podCount)
	ds.Greater(podCount, 0, "expecting to see few pods in kube-system namespace")
	//
	ds.T().Log("waiting to see calico pods ready")
	ds.NoError(common.WaitForCalicoReady(kc), "calico did not start")
}
