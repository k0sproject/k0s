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

package disabledcomponents

import (
	"fmt"
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
	_, err := ssh.ExecWithOutput(s.Context(), fmt.Sprintf("pidof %s", procName))
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
