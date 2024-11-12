/*
Copyright 2020 k0s authors

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

package kine

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
	"k8s.io/apimachinery/pkg/util/wait"

	"github.com/k0sproject/k0s/inttest/common"
)

type KineSuite struct {
	common.BootlooseSuite
}

func (s *KineSuite) TestK0sGetsUp() {
	s.PutFile(s.ControllerNode(0), "/tmp/k0s.yaml", k0sConfigWithKine)
	s.NoError(s.InitController(0, "--config=/tmp/k0s.yaml", "--enable-metrics-scraper"))
	s.NoError(s.RunWorkers())

	kc, err := s.KubeClient(s.ControllerNode(0))
	s.Require().NoError(err)

	err = s.WaitForNodeReady(s.WorkerNode(0), kc)
	s.NoError(err)

	err = s.WaitForNodeReady(s.WorkerNode(1), kc)
	s.NoError(err)

	s.AssertSomeKubeSystemPods(kc)

	s.T().Log("waiting to see CNI pods ready")
	s.NoError(common.WaitForKubeRouterReady(s.Context(), kc), "CNI did not start")

	s.Run("verify", func() {
		ssh, err := s.SSH(s.Context(), s.ControllerNode(0))
		s.Require().NoError(err, "failed to SSH into controller")
		defer ssh.Disconnect()

		s.Run(("kineIsUsedAsStorage"), func() {
			_, err = ssh.ExecWithOutput(s.Context(), "test -e /var/lib/k0s/bin/kine && ps xa | grep kine")
			s.NoError(err)
		})

		s.Run(("noControllerJoinTokens"), func() {
			noToken, err := ssh.ExecWithOutput(s.Context(), fmt.Sprintf("'%s' token create --role=controller", s.K0sFullPath))
			s.Error(err)
			s.Equal("Error: refusing to create token: cannot join controller into current storage", noToken)
		})

		s.Run(("workerJoinTokens"), func() {
			_, err := ssh.ExecWithOutput(s.Context(), fmt.Sprintf("'%s' token create --role=worker", s.K0sFullPath))
			s.NoError(err)
		})
	})

	s.Run("metrics", func() {
		s.Require().NoError(common.WaitForDeployment(s.Context(), kc, "k0s-pushgateway", "k0s-system"))
		s.Require().NoError(wait.PollImmediateInfiniteWithContext(s.Context(), 5*time.Second, func(ctx context.Context) (bool, error) {
			b, err := kc.RESTClient().Get().AbsPath("/api/v1/namespaces/k0s-system/services/http:k0s-pushgateway:http/proxy/metrics").DoRaw(s.Context())
			if err != nil {
				return false, nil
			}

			// wait for kube-scheduler and kube-controller-manager metrics
			output := string(b)
			return strings.Contains(output, `job="kube-scheduler"`) && strings.Contains(output, `job="kube-controller-manager"`) && strings.Contains(output, `job="kine"`), nil
		}))
	})
}

func TestKineSuite(t *testing.T) {
	s := KineSuite{
		common.BootlooseSuite{
			ControllerCount: 1,
			WorkerCount:     2,
		},
	}
	suite.Run(t, &s)
}

const k0sConfigWithKine = `
spec:
  storage:
    type: kine
`
