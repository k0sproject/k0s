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

const configWithCustomPorts = `
apiVersion: k0s.k0sproject.io/v1beta1
kind: Cluster
metadata:
  name: k0s
spec:
  api:
    port: %d
    k0s_api_port: %d
  konnectivity:
    agent_port: %d
    admin_port: %d
`

const apiPort = 7443
const k0sApiPort = 9743
const agentPort = 9132
const adminPort = 9133

func TestSuite(t *testing.T) {

	s := Suite{
		common.FootlooseSuite{
			ControllerCount: 1,
			WorkerCount:     1,
		},
		nil,
	}
	suite.Run(t, &s)
}

func (ds *Suite) TestWorkerJoinsWithCustomPort() {
	dataDir := "/var/lib/k0s"
	ds.createConfigWithCustomPorts("controller0", "/tmp/k0s.yaml", apiPort, k0sApiPort, agentPort, adminPort)
	ds.createConfigWithCustomPorts("worker0", "/tmp/k0s.yaml", apiPort, k0sApiPort, agentPort, adminPort)

	ds.NoError(ds.InitMainController([]string{"--config=/tmp/k0s.yaml"}))
	kc, err := ds.KubeClient("controller0", dataDir)
	ds.NoError(err)
	token, err := ds.GetJoinToken("worker", dataDir, "--config=/tmp/k0s.yaml")
	ds.NoError(err)
	ds.NoError(ds.RunWorkersWithToken(dataDir, token, `--config="/tmp/k0s.yaml"`))

	ds.NoError(ds.WaitForNodeReady("worker0", kc))

	pods, err := kc.CoreV1().Pods("kube-system").List(context.TODO(), v1.ListOptions{
		Limit: 100,
	})
	ds.NoError(err)

	podCount := len(pods.Items)

	ds.T().Logf("found %d pods in kube-system", podCount)
	ds.Greater(podCount, 0, "expecting to see few pods in kube-system namespace")

}

func (ds *Suite) createConfigWithCustomPorts(node string, configPath string, apiPort, k0sApiPort, agentPort, adminPort int) {
	ssh, err := ds.SSH(node)
	ds.Require().NoError(err)
	defer ssh.Disconnect()
	_, err = ssh.ExecWithOutput(fmt.Sprintf("echo '%s' >%s", fmt.Sprintf(configWithCustomPorts, apiPort, k0sApiPort, agentPort, adminPort), configPath))

	ds.Require().NoError(err)
}

func (ds *Suite) TestControllerJoinsWithCustomPort() {
	//nl, err := ds.client.CoreV1().Nodes().List(context.Background(), v1meta.ListOptions{})
	//ds.Require().NoError(err)
	//for _, n := range nl.Items {
	//	ds.Require().Len(n.Spec.PodCIDRs, 2, "Each node must have ipv4 and ipv6 pod cidr")
	//}
}
