// SPDX-FileCopyrightText: 2022 k0s authors
// SPDX-License-Identifier: Apache-2.0

package single

import (
	"os"
	"strings"
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

	// customStatusSocketPath is a non-default socket location used by the
	// "ap-single-custom-socket" target to verify that autopilot signal
	// controllers honor --status-socket. Default would be /run/k0s/status.sock.
	customStatusSocketPath = "/run/k0s/custom/status.sock"
	// customDataDir places the data dir (and therefore many derived paths) in
	// a non-default location, exercising the wider --data-dir surface.
	customDataDir = "/var/lib/k0s-custom"
)

// useCustomStatusSocket reports whether the suite was invoked through the
// "ap-single-custom-socket" check target. Driven by K0S_INTTEST_TARGET, the
// same env-var convention used by other parameterized inttests
// (cplb-userspace, dualstack, network-conformance, ...).
func useCustomStatusSocket() bool {
	return strings.Contains(os.Getenv("K0S_INTTEST_TARGET"), "custom-socket")
}

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

	args := []string{"--single", "--disable-components=metrics-server"}
	if useCustomStatusSocket() {
		// Pre-create the parent directories so k0s doesn't have to mkdir at
		// odd paths during early boot. These match the flags below.
		require.NoError(ssh.Exec(ctx, "mkdir -p /run/k0s/custom "+customDataDir, common.SSHStreams{}))
		args = append(args,
			"--status-socket="+customStatusSocketPath,
			"--data-dir="+customDataDir,
		)
		s.T().Logf("Launching controller with custom status socket %q and data dir %q",
			customStatusSocketPath, customDataDir)
	}
	require.NoError(s.InitController(0, args...))

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

	if useCustomStatusSocket() {
		// Defense against #3719: an AP test can report PlanCompleted even
		// when the cluster is unhealthy. Cross-check that:
		//   1. The custom status socket was actually created (the controller
		//      honored --status-socket).
		//   2. The default /run/k0s/status.sock was NOT created (no caller
		//      fell back to the hardcoded path).
		// If autopilot's signal controllers had still hardcoded the default
		// path (the bug this PR fixes), step 2 would fail because the
		// post-restart status probe would either create or contact the
		// default socket location.
		ssh, err := s.SSH(ctx, s.ControllerNode(0))
		s.Require().NoError(err)
		defer ssh.Disconnect()

		s.NoError(ssh.Exec(ctx, "test -S "+customStatusSocketPath, common.SSHStreams{}),
			"custom status socket %q should exist", customStatusSocketPath)
		s.Error(ssh.Exec(ctx, "test -e /run/k0s/status.sock", common.SSHStreams{}),
			"default status socket /run/k0s/status.sock should NOT exist when --status-socket is set")
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
