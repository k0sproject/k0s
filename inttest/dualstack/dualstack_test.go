/*
Copyright 2022 k0s authors

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

package dualstack

// Package implements basic smoke test for dualstack setup.
// Since we run tests under containers environment in the GHA we can't
// actually check proper network connectivity.
// Until wi migrate toward VM based test suites
// this test only checks that nodes in the cluster
// have proper values for spec.PodCIDRs

import (
	"fmt"
	"os"
	"strings"

	"github.com/stretchr/testify/suite"
	v1meta "k8s.io/apimachinery/pkg/apis/meta/v1"

	"testing"

	"github.com/k0sproject/k0s/inttest/common"
	k8s "k8s.io/client-go/kubernetes"
)

type DualstackSuite struct {
	common.FootlooseSuite

	client *k8s.Clientset
}

func (s *DualstackSuite) TestDualStackNodesHavePodCIDRs() {
	nl, err := s.client.CoreV1().Nodes().List(s.Context(), v1meta.ListOptions{})
	s.Require().NoError(err)
	for _, n := range nl.Items {
		s.Require().Len(n.Spec.PodCIDRs, 2, "Each node must have ipv4 and ipv6 pod cidr")
	}
}

func (s *DualstackSuite) TestDualStackControlPlaneComponentsHaveServiceCIDRs() {
	const expected = "--service-cluster-ip-range=10.96.0.0/12,fd01::/108"
	node := s.ControllerNode(0)

	s.Contains(s.cmdlineForExecutable(node, "kube-apiserver"), expected)
	s.Contains(s.cmdlineForExecutable(node, "kube-controller-manager"), expected)
}

func (s *DualstackSuite) cmdlineForExecutable(node, binary string) []string {
	require := s.Require()
	ssh, err := s.SSH(node)
	require.NoError(err)
	defer ssh.Disconnect()

	output, err := ssh.ExecWithOutput(s.Context(), fmt.Sprintf("pidof -- %q", binary))
	require.NoError(err)

	pids := strings.Split(output, " ")
	require.Len(pids, 1, "Expected a single pid")

	output, err = ssh.ExecWithOutput(s.Context(), fmt.Sprintf("cat /proc/%q/cmdline", pids[0]))
	require.NoError(err, "Failed to get cmdline for PID %s", pids[0])
	return strings.Split(output, "\x00")
}

func (s *DualstackSuite) SetupSuite() {
	s.FootlooseSuite.SetupSuite()
	s.PutFile(s.ControllerNode(0), "/tmp/k0s.yaml", k0sConfigWithDualStack)
	controllerArgs := []string{"--config=/tmp/k0s.yaml"}
	if os.Getenv("K0S_ENABLE_DYNAMIC_CONFIG") == "true" {
		s.T().Log("Enabling dynamic config for controller")
		controllerArgs = append(controllerArgs, "--enable-dynamic-config")
	}
	s.Require().NoError(s.InitController(0, controllerArgs...))
	s.Require().NoError(s.RunWorkers())
	client, err := s.KubeClient(s.ControllerNode(0))
	s.Require().NoError(err)
	err = s.WaitForNodeReady(s.WorkerNode(0), client)
	s.Require().NoError(err)

	err = s.WaitForNodeReady(s.WorkerNode(1), client)
	s.Require().NoError(err)

	s.client = client
}

func TestDualStack(t *testing.T) {

	s := DualstackSuite{
		common.FootlooseSuite{
			ControllerCount: 1,
			WorkerCount:     2,
		},
		nil,
	}

	suite.Run(t, &s)

}

const k0sConfigWithDualStack = `
spec:
  network:
    kubeProxy:
      mode: ipvs
    provider: calico
    calico:
      mode: "bird"
    dualStack:
      enabled: true
      IPv6podCIDR: "fd00::/108"
      IPv6serviceCIDR: "fd01::/108"
    podCIDR: 10.244.0.0/16
    serviceCIDR: 10.96.0.0/12`
