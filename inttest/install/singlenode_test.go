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
package install

import (
	"context"
	"testing"

	"github.com/k0sproject/k0s/inttest/common"
	"github.com/stretchr/testify/suite"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type InstallSuite struct {
	common.FootlooseSuite
}

func (s *InstallSuite) TestK0sGetsUp() {
	ssh, err := s.SSH(s.ControllerNode(0))
	s.Require().NoError(err)
	defer ssh.Disconnect()

	_, err = ssh.ExecWithOutput("k0s install controller --enable-worker")
	s.Require().NoError(err)

	_, err = ssh.ExecWithOutput("rc-service k0scontroller start")
	s.Require().NoError(err)

	err = s.WaitForKubeAPI(s.ControllerNode(0), "")
	s.Require().NoError(err)

	kc, err := s.KubeClient(s.ControllerNode(0), "")
	s.NoError(err)

	err = s.WaitForNodeReady(s.ControllerNode(0), kc)
	s.NoError(err)

	pods, err := kc.CoreV1().Pods("kube-system").List(context.TODO(), v1.ListOptions{
		Limit: 100,
	})
	s.NoError(err)

	podCount := len(pods.Items)

	s.T().Logf("found %d pods in kube-system", podCount)
	s.Greater(podCount, 0, "expecting to see few pods in kube-system namespace")

	s.T().Log("waiting to see calico pods ready")
	s.NoError(common.WaitForCalicoReady(kc), "calico did not start")

	s.T().Log("running k0s reset command")

	_, err = ssh.ExecWithOutput("rc-service k0scontroller stop")
	s.Require().NoError(err)

	_, err = ssh.ExecWithOutput("k0s reset --debug")
	s.Require().NoError(err)

	fileCount, err := ssh.ExecWithOutput("find /var/lib/k0s -type f | wc -l")
	s.Equal("0", fileCount, "expected to see 0 files under /var/lib/k0s")
	s.Require().NoError(err)
}

func TestInstallSuite(t *testing.T) {
	s := InstallSuite{
		common.FootlooseSuite{
			ControllerCount: 1,
		},
	}
	suite.Run(t, &s)
}
