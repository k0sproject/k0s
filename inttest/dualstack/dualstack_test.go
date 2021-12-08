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

package dualstack

// Package implements basic smoke test for dualstack setup.
// Since we run tests under containers environment in the GHA we can't
// actually check proper network connectivity.
// Until wi migrate toward VM based test suites
// this test only checks that nodes in the cluster
// have proper values for spec.PodCIDRs

import (
	"context"
	"fmt"

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

func (ds *DualstackSuite) TestDualStackNodesHavePodCIDRs() {
	nl, err := ds.client.CoreV1().Nodes().List(context.Background(), v1meta.ListOptions{})
	ds.Require().NoError(err)
	for _, n := range nl.Items {
		ds.Require().Len(n.Spec.PodCIDRs, 2, "Each node must have ipv4 and ipv6 pod cidr")
	}
}

func (ds *DualstackSuite) TestDualStackControlPlaneComponentsHaveServiceCIDRs() {
	err := ds.verifyKubeApiServiceClusterIPRangeFlag(ds.ControllerNode(0))
	ds.Require().NoError(err)
	err = ds.verifyKubeControllerManagerServiceClusterIPRangeFlag(ds.ControllerNode(0))
	ds.Require().NoError(err)
}

// Verifies that kube-apiserver process has a dual-stack service-cluster-ip-range configured.
func (s *DualstackSuite) verifyKubeApiServiceClusterIPRangeFlag(node string) error {
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
		return fmt.Errorf("kube-apiserver does not have proper a dual-stack service-cluster-ip-range set.")
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
		return fmt.Errorf("kube-controller-manager does not have proper a dual-stack service-cluster-ip-range set.")
	}

	return nil
}

func (ds *DualstackSuite) SetupSuite() {
	ds.FootlooseSuite.SetupSuite()
	ds.PutFile(ds.ControllerNode(0), "/tmp/k0s.yaml", k0sConfigWithDualStack)
	ds.Require().NoError(ds.InitController(0, "--config=/tmp/k0s.yaml"))
	ds.Require().NoError(ds.RunWorkers())
	client, err := ds.KubeClient(ds.ControllerNode(0))
	ds.Require().NoError(err)
	err = ds.WaitForNodeReady(ds.WorkerNode(0), client)
	ds.Require().NoError(err)

	err = ds.WaitForNodeReady(ds.WorkerNode(1), client)
	ds.Require().NoError(err)

	ds.client = client

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
