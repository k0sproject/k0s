// SPDX-FileCopyrightText: 2021 k0s authors
// SPDX-License-Identifier: Apache-2.0

package disabledcomponents

import (
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/k0sproject/k0s/inttest/common"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type DisabledComponentsSuite struct {
	common.BootlooseSuite
}

func (s *DisabledComponentsSuite) TestK0sGetsUp() {

	s.NoError(s.InitController(0, "--disable-components control-api,coredns,csr-approver,helm,konnectivity-server,kube-controller-manager,kube-proxy,kube-scheduler,metrics-server,network-provider,system-rbac,worker-config"))

	kc, err := s.KubeClient(s.ControllerNode(0))
	s.Require().NoError(err)

	if pods, err := kc.CoreV1().Pods("kube-system").List(s.Context(), v1.ListOptions{
		Limit: 100,
	}); s.NoError(err) {
		s.Empty(pods.Items, "Expected to see no pods in kube-system namespace")
	}

	ssh, err := s.SSH(s.Context(), s.ControllerNode(0))
	s.Require().NoError(err)
	defer ssh.Disconnect()
	s.True(s.processExists("kube-apiserver", ssh))
	s.False(s.processExists("konnectivity-server", ssh))
	s.False(s.processExists("kube-scheduler", ssh))
	s.False(s.processExists("kube-controller-manager", ssh))
}

func (s *DisabledComponentsSuite) processExists(procName string, ssh *common.SSHConnection) bool {
	_, err := ssh.ExecWithOutput(s.Context(), "pidof "+procName)
	return err == nil // `pidof xyz` return 1 if the process does not exist
}

func TestDisabledComponentsSuite(t *testing.T) {
	s := DisabledComponentsSuite{
		common.BootlooseSuite{
			ControllerCount: 1,
			WorkerCount:     0,
		},
	}
	suite.Run(t, &s)
}
