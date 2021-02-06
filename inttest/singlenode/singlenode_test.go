/*
Copyright 2020 Mirantis, Inc.

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
package singlenode

import (
	"context"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/k0sproject/k0s/inttest/common"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type SingleNodeSuite struct {
	common.FootlooseSuite
}

func (s *SingleNodeSuite) TestK0sGetsUp() {
	ssh, err := s.SSH("controller0")
	s.Require().NoError(err)
	defer ssh.Disconnect()

	_, err = ssh.ExecWithOutput("ETCD_UNSUPPORTED_ARCH=arm64 nohup k0s --debug server --enable-worker >/tmp/k0s-server.log 2>&1 &")
	s.Require().NoError(err)
	err = s.WaitForKubeAPI("controller0", "")
	s.Require().NoError(err)

	kc, err := s.KubeClient("controller0", "")
	s.NoError(err)

	err = s.WaitForNodeReady("controller0", kc)
	s.NoError(err)

	// Verify we get the Ready=true status for the node through embedded kubectl command
	status, err := ssh.ExecWithOutput(`k0s kubectl get node controller0 -o jsonpath='{.status.conditions[?(@.type=="Ready")].status}'`)
	s.Require().NoError(err)
	s.Require().Equal("True", status)

	pods, err := kc.CoreV1().Pods("kube-system").List(context.TODO(), v1.ListOptions{
		Limit: 100,
	})
	s.NoError(err)

	podCount := len(pods.Items)

	s.T().Logf("found %d pods in kube-system", podCount)
	s.Greater(podCount, 0, "expecting to see few pods in kube-system namespace")

	s.T().Log("waiting to see calico pods ready")
	s.NoError(common.WaitForCalicoReady(kc), "calico did not start")
}

func TestSingleNodeSuite(t *testing.T) {
	s := SingleNodeSuite{
		common.FootlooseSuite{
			ControllerCount: 1,
		},
	}
	suite.Run(t, &s)
}
