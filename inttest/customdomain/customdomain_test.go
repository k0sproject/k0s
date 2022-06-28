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
package customdomain

import (
	"context"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/k0sproject/k0s/inttest/common"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type CustomDomainSuite struct {
	common.FootlooseSuite
}

func (s *CustomDomainSuite) TestK0sGetsUpWithCustomDomain() {
	s.PutFile(s.ControllerNode(0), "/tmp/k0s.yaml", k0sConfig)
	// Metrics disabled as it's super slow to get up properly and interferes with API discovery etc. while it's getting up
	s.NoError(s.InitController(0, "--config=/tmp/k0s.yaml", "--disable-components metrics-server"))
	s.NoError(s.RunWorkers())

	kc, err := s.KubeClient(s.ControllerNode(0))
	s.Require().NoError(err)

	err = s.WaitForNodeReady(s.WorkerNode(0), kc)
	s.NoError(err)

	err = s.WaitForNodeReady(s.WorkerNode(1), kc)
	s.NoError(err)

	pods, err := kc.CoreV1().Pods("kube-system").List(context.TODO(), v1.ListOptions{
		Limit: 100,
	})
	s.NoError(err)

	podCount := len(pods.Items)

	s.T().Logf("found %d pods in kube-system", podCount)
	s.Greater(podCount, 0, "expecting to see few pods in kube-system namespace")

	s.T().Log("waiting to see CNI pods ready")
	s.NoError(common.WaitForKubeRouterReadyWithContext(s.Context(), kc), "CNI did not start")

	s.T().Run("check custom domain existence in pod", func(t *testing.T) {
		// All done via SSH as it's much simpler :)
		// e.g. execing via client-go is super complex and would require too much wiring
		ssh, err := s.SSH(s.ControllerNode(0))
		s.NoError(err)
		_, err = ssh.ExecWithOutput("/usr/local/bin/k0s kc run nginx --image docker.io/nginx:1-alpine")
		s.NoError(err)
		s.NoError(common.WaitForPod(kc, "nginx", "default"))
		output, err := ssh.ExecWithOutput("/usr/local/bin/k0s kc exec nginx -- cat /etc/resolv.conf")
		s.NoError(err)
		s.Contains(output, "search default.svc.something.local svc.something.local something.local")
	})
}

func TestCustomDomainSuite(t *testing.T) {
	s := CustomDomainSuite{
		common.FootlooseSuite{
			ControllerCount: 1,
			WorkerCount:     2,
		},
	}
	suite.Run(t, &s)
}

const k0sConfig = `
spec:
  storage:
    type: kine
  network:
    clusterDomain: something.local
`
