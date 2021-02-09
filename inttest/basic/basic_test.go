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
package basic

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/k0sproject/k0s/inttest/common"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type BasicSuite struct {
	common.FootlooseSuite
}

func (s *BasicSuite) TestK0sGetsUp() {
	customDataDir := "/var/lib/k0s/custom-data-dir"
	s.NoError(s.InitMainController("/tmp/k0s.yaml", customDataDir))
	s.NoError(s.RunWorkers(customDataDir))

	kc, err := s.KubeClient("controller0", customDataDir)
	s.NoError(err)

	err = s.WaitForNodeReady("worker0", kc)
	s.NoError(err)

	labels, err := s.GetNodeLabels("worker0", kc)
	s.NoError(err)
	s.Equal("bar", labels["k0sproject.io/foo"])

	err = s.WaitForNodeReady("worker1", kc)
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

	s.Require().NoError(s.checkCertPerms("controller0"))
}

func (s *BasicSuite) checkCertPerms(node string) error {
	ssh, err := s.SSH(node)
	if err != nil {
		return err
	}
	defer ssh.Disconnect()

	output, err := ssh.ExecWithOutput(`find /var/lib/k0s/custom-data-dir/pki/  \( -name '*.key' -o -name '*.conf' \) -a \! -perm 0640`)
	if err != nil {
		return err
	}

	if output != "" {
		return fmt.Errorf("some private files having non 640 permissions: %s", output)
	}

	return nil
}

func TestBasicSuite(t *testing.T) {
	s := BasicSuite{
		common.FootlooseSuite{
			ControllerCount: 1,
			WorkerCount:     2,
		},
	}
	suite.Run(t, &s)
}
