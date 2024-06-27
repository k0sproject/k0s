// Copyright 2024 k0s authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package controllerworker

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/k0sproject/k0s/inttest/common"
	aptest "github.com/k0sproject/k0s/inttest/common/autopilot"

	apconst "github.com/k0sproject/k0s/pkg/autopilot/constant"
	appc "github.com/k0sproject/k0s/pkg/autopilot/controller/plans/core"
	"github.com/k0sproject/k0s/pkg/constant"
	"github.com/k0sproject/k0s/pkg/kubernetes/watch"

	"github.com/stretchr/testify/suite"

	corev1 "k8s.io/api/core/v1"
)

type controllerworkerSuite struct {
	common.BootlooseSuite
}

const k0sConfigWithMultiController = `
spec:
  api:
    address: %s
  storage:
    etcd:
      peerAddress: %s
`

// SetupTest prepares the controller and filesystem, getting it into a consistent
// state which we can run tests against.
func (s *controllerworkerSuite) SetupTest() {
	ctx := s.Context()
	// ipAddress := s.GetControllerIPAddress(0)
	var joinToken string

	for idx := 0; idx < s.BootlooseSuite.ControllerCount; idx++ {
		nodeName, require := s.ControllerNode(idx), s.Require()
		address := s.GetControllerIPAddress(idx)

		s.Require().NoError(s.WaitForSSH(nodeName, 2*time.Minute, 1*time.Second))
		ssh, err := s.SSH(ctx, nodeName)
		require.NoError(err)
		defer ssh.Disconnect()
		s.PutFile(nodeName, "/tmp/k0s.yaml", fmt.Sprintf(k0sConfigWithMultiController, address, address))

		// Note that the token is intentionally empty for the first controller
		args := []string{
			"--debug",
			"--disable-components=metrics-server,helm,konnectivity-server",
			"--enable-worker",
			"--config=/tmp/k0s.yaml",
		}
		if joinToken != "" {
			s.PutFile(nodeName, "/tmp/token", joinToken)
			args = append(args, "--token-file=/tmp/token")
		}
		out, err := ssh.ExecWithOutput(ctx, "cp -f /dist/k0s /usr/local/bin/k0s && /usr/local/bin/k0s install controller "+strings.Join(args, " "))
		if err != nil {
			s.T().Logf("error installing k0s: %s", out)
		}
		require.NoError(err)
		_, err = ssh.ExecWithOutput(ctx, "k0s start")
		require.NoError(err)
		// s.Require().NoError(s.InitController(idx, "--config=/tmp/k0s.yaml", "--disable-components=metrics-server", "--enable-worker", joinToken))
		s.Require().NoError(s.WaitJoinAPI(nodeName))
		kc, err := s.KubeClient(nodeName)
		require.NoError(err)
		require.NoError(s.WaitForNodeReady(nodeName, kc))

		client, err := s.ExtensionsClient(s.ControllerNode(0))
		s.Require().NoError(err)

		s.Require().NoError(aptest.WaitForCRDByName(ctx, client, "plans"))
		s.Require().NoError(aptest.WaitForCRDByName(ctx, client, "controlnodes"))

		// With the primary controller running, create the join token for subsequent controllers.
		if idx == 0 {
			token, err := s.GetJoinToken("controller")
			s.Require().NoError(err)
			joinToken = token
		}
	}

	// Final sanity -- ensure all nodes see each other according to etcd
	for idx := 0; idx < s.BootlooseSuite.ControllerCount; idx++ {
		s.Require().Len(s.GetMembers(idx), s.BootlooseSuite.ControllerCount)
	}
}

// TestApply applies a well-formed `plan` yaml, and asserts that
// all of the correct values across different objects + controllers are correct.
func (s *controllerworkerSuite) TestApply() {

	planTemplate := `
apiVersion: autopilot.k0sproject.io/v1beta2
kind: Plan
metadata:
  name: autopilot
spec:
  id: id123
  timestamp: now
  commands:
    - k0supdate:
        version: v0.0.0
        forceupdate: true
        platforms:
          linux-amd64:
            url: http://localhost/dist/k0s-new
          linux-arm64:
            url: http://localhost/dist/k0s-new
        targets:
          controllers:
            discovery:
              static:
                nodes:
                  - controller1
                  - controller2
                  - controller0
`
	ctx := s.Context()
	manifestFile := "/tmp/happy.yaml"
	s.PutFileTemplate(s.ControllerNode(0), manifestFile, planTemplate, nil)

	out, err := s.RunCommandController(0, fmt.Sprintf("/usr/local/bin/k0s kubectl apply -f %s", manifestFile))
	s.T().Logf("kubectl apply output: '%s'", out)
	s.Require().NoError(err)

	client, err := s.AutopilotClient(s.ControllerNode(0))
	s.Require().NoError(err)
	s.NotEmpty(client)

	// The plan has enough information to perform a successful update of k0s, so wait for it.
	plan, err := aptest.WaitForPlanState(s.Context(), client, apconst.AutopilotName, appc.PlanCompleted)
	s.Require().NoError(err)

	s.Equal(1, len(plan.Status.Commands))
	cmd := plan.Status.Commands[0]

	s.Equal(appc.PlanCompleted, cmd.State)
	s.NotNil(cmd.K0sUpdate)
	s.NotNil(cmd.K0sUpdate.Controllers)
	s.Empty(cmd.K0sUpdate.Workers)

	for _, node := range cmd.K0sUpdate.Controllers {
		s.Equal(appc.SignalCompleted, node.State)
	}

	kc, err := s.KubeClient(s.ControllerNode(0))
	s.NoError(err)

	for idx := 0; idx < s.BootlooseSuite.ControllerCount; idx++ {
		nodeName, require := s.ControllerNode(idx), s.Require()
		require.NoError(s.WaitForNodeReady(nodeName, kc))
		// Wait till we see kubelet reporting the expected version.
		// This is only bullet proof if upgrading to _another_ Kubernetes version.
		err := watch.Nodes(kc.CoreV1().Nodes()).
			WithObjectName(nodeName).
			WithErrorCallback(common.RetryWatchErrors(s.T().Logf)).
			Until(ctx, func(node *corev1.Node) (bool, error) {
				return strings.Contains(node.Status.NodeInfo.KubeletVersion, fmt.Sprintf("v%s.", constant.KubernetesMajorMinorVersion)), nil
			})
		require.NoError(err)
	}
}

// TestQuorumSuite sets up a suite using 3 controllers for quorum, and runs various
// autopilot upgrade scenarios against them.
func TestQuorumSuite(t *testing.T) {
	suite.Run(t, &controllerworkerSuite{
		common.BootlooseSuite{
			ControllerCount: 3,
			WorkerCount:     0,
			LaunchMode:      common.LaunchModeOpenRC,
		},
	})
}
