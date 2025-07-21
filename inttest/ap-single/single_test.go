// SPDX-FileCopyrightText: 2022 k0s authors
// SPDX-License-Identifier: Apache-2.0

package single

import (
	"testing"

	apconst "github.com/k0sproject/k0s/pkg/autopilot/constant"
	appc "github.com/k0sproject/k0s/pkg/autopilot/controller/plans/core"
	k0sclientset "github.com/k0sproject/k0s/pkg/client/clientset"

	"github.com/k0sproject/k0s/inttest/common"
	aptest "github.com/k0sproject/k0s/inttest/common/autopilot"

	"github.com/stretchr/testify/suite"
)

const (
	ManifestTestDirPerms = "775"
)

type plansSingleControllerSuite struct {
	common.BootlooseSuite
}

// SetupTest prepares the controller and filesystem, getting it into a consistent
// state which we can run tests against.
func (s *plansSingleControllerSuite) SetupTest() {
	ctx, nodeName, require := s.Context(), s.ControllerNode(0), s.Require()

	// Move the k0s binary to a new location, so we can check the binary location detection
	ssh, err := s.SSH(ctx, nodeName)
	require.NoError(err)
	defer ssh.Disconnect()
	require.NoError(ssh.Exec(ctx, "cp /dist/k0s /tmp/k0s", common.SSHStreams{}))

	require.NoError(s.InitController(0, "--single", "--disable-components=metrics-server"))

	client, err := s.KubeClient(nodeName)
	require.NoError(err)
	require.NoError(s.WaitForNodeReady(nodeName, client))

	xClient, err := s.ExtensionsClient(nodeName)
	require.NoError(err)
	require.NoError(aptest.WaitForCRDByName(ctx, xClient, "plans"))
	require.NoError(aptest.WaitForCRDByName(ctx, xClient, "controlnodes"))
}

// TestApply applies a well-formed `plan` yaml, and asserts that all of the correct values
// across different objects are correct.
func (s *plansSingleControllerSuite) TestApply() {
	ctx := s.Context()

	restConfig, err := s.GetKubeConfig(s.ControllerNode(0))
	s.Require().NoError(err)

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
            url: http://localhost/dist/k0s
        targets:
          controllers:
            discovery:
              static:
                nodes:
                  - controller0
`

	_, err = common.Create(ctx, restConfig, []byte(planTemplate))
	s.Require().NoError(err)
	s.T().Logf("Plan created")

	// The plan has enough information to perform a successful update of k0s, so wait for it.
	client, err := k0sclientset.NewForConfig(restConfig)
	s.Require().NoError(err)
	plan, err := aptest.WaitForPlanState(ctx, client, apconst.AutopilotName, appc.PlanCompleted)
	s.Require().NoError(err, "While waiting for plan to complete")

	if s.Len(plan.Status.Commands, 1) {
		cmd := plan.Status.Commands[0]

		s.Equal(appc.PlanCompleted, cmd.State)
		s.NotNil(cmd.K0sUpdate)
		s.NotNil(cmd.K0sUpdate.Controllers)
		s.Empty(cmd.K0sUpdate.Workers)
		s.Equal(appc.SignalCompleted, cmd.K0sUpdate.Controllers[0].State)
	}
}

// TestPlansSingleControllerSuite sets up a suite using a single controller, running various
// autopilot upgrade scenarios against it.
func TestPlansSingleControllerSuite(t *testing.T) {
	suite.Run(t, &plansSingleControllerSuite{
		common.BootlooseSuite{
			K0sFullPath:     "/tmp/k0s",
			ControllerCount: 1,
			WorkerCount:     0,
			LaunchMode:      common.LaunchModeOpenRC,
		},
	})
}
