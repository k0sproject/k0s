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
package disabledcomponents

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/k0sproject/k0s/inttest/common"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type DisabledComponentsSuite struct {
	common.FootlooseSuite
}

func (s *DisabledComponentsSuite) TestK0sGetsUp() {

	s.NoError(s.InitController(0, "--disable-components konnectivity-server,kube-scheduler,kube-controller-manager,control-api,csr-approver,default-psp,kube-proxy,coredns,network-provider,helm,metrics-server,kubelet-config,system-rbac"))

	kc, err := s.KubeClient(s.ControllerNode(0))
	s.NoError(err)

	pods, err := kc.CoreV1().Pods("kube-system").List(context.TODO(), v1.ListOptions{
		Limit: 100,
	})
	s.NoError(err)

	podCount := len(pods.Items)

	s.T().Logf("found %d pods in kube-system", podCount)
	s.Equal(podCount, 0, "expecting to see few pods in kube-system namespace")

	ssh, err := s.SSH(s.ControllerNode(0))
	s.NoError(err)
	s.True(s.processExists("kube-apiserver", ssh))
	s.False(s.processExists("konnectivity-server", ssh))
	s.False(s.processExists("kube-scheduler", ssh))
	s.False(s.processExists("kube-controller-manager", ssh))
}

func (s *DisabledComponentsSuite) processExists(procName string, ssh *common.SSHConnection) bool {
	_, err := ssh.ExecWithOutput(fmt.Sprintf("pidof %s", procName))
	return err == nil // `pidof xyz` return 1 if the process does not exist
}

func TestDisabledComponentsSuite(t *testing.T) {
	s := DisabledComponentsSuite{
		common.FootlooseSuite{
			ControllerCount: 1,
			WorkerCount:     0,
		},
	}
	suite.Run(t, &s)
}
