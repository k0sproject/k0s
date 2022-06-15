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
	"context"
	"errors"

	"github.com/stretchr/testify/suite"
	v1meta "k8s.io/apimachinery/pkg/apis/meta/v1"

	"testing"

	"github.com/k0sproject/k0s/tests/smoke/common"
	k8s "k8s.io/client-go/kubernetes"
)

type DualstackSuite struct {
	common.FootlooseSuite

	client *k8s.Clientset
}

func (s *DualstackSuite) TestDualStackNodesHavePodCIDRs() {
	nl, err := s.client.CoreV1().Nodes().List(context.Background(), v1meta.ListOptions{})
	s.Require().NoError(err)
	for _, n := range nl.Items {
		s.Require().Len(n.Spec.PodCIDRs, 2, "Each node must have ipv4 and ipv6 pod cidr")
	}
}

func (s *DualstackSuite) TestDualStackControlPlaneComponentsHaveServiceCIDRs() {
	err := s.verifyKubeAPIServiceClusterIPRangeFlag(s.ControllerNode(0))
	s.Require().NoError(err)
	err = s.verifyKubeControllerManagerServiceClusterIPRangeFlag(s.ControllerNode(0))
	s.Require().NoError(err)
}

// Verifies that kube-apiserver process has a dual-stack service-cluster-ip-range configured.
func (s *DualstackSuite) verifyKubeAPIServiceClusterIPRangeFlag(node string) error {
	ssh, err := s.SSH(node)
	if err != nil {
		return err
	}
	defer ssh.Disconnect()

	output, err := ssh.ExecWithOutput(`grep -e '--service-cluster-ip-range=10.96.0.0/12,fd01::/108' /proc/$(pidof kube-apiserver)/cmdline`)
	if err != nil {
		return err
	}
	if output != "--service-cluster-ip-range=10.96.0.0/12,fd01::/108" {
		return errors.New("kube-apiserver does not have proper a dual-stack service-cluster-ip-range set")
	}

	return nil
}

// Verifies that kube-controller-manager process has a dual-stack service-cluster-ip-range configured.
func (s *DualstackSuite) verifyKubeControllerManagerServiceClusterIPRangeFlag(node string) error {
	ssh, err := s.SSH(node)
	if err != nil {
		return err
	}
	defer ssh.Disconnect()

	output, err := ssh.ExecWithOutput(`grep -e '--service-cluster-ip-range=10.96.0.0/12,fd01::/108' /proc/$(pidof kube-controller-manager)/cmdline`)
	if err != nil {
		return err
	}
	if output != "--service-cluster-ip-range=10.96.0.0/12,fd01::/108" {
		return errors.New("kube-controller-manager does not have proper a dual-stack service-cluster-ip-range set")
	}

	return nil
}

func (s *DualstackSuite) SetupSuite() {
	s.FootlooseSuite.SetupSuite()
	s.PutFile(s.ControllerNode(0), "/tmp/k0s.yaml", k0sConfigWithDualStack)
	s.Require().NoError(s.InitController(0, "--config=/tmp/k0s.yaml"))
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
