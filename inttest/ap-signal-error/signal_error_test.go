// SPDX-FileCopyrightText: 2026 k0s authors
// SPDX-License-Identifier: Apache-2.0

package signalerror

import (
	"testing"

	"github.com/k0sproject/k0s/inttest/common"
	aptest "github.com/k0sproject/k0s/inttest/common/autopilot"
	apconst "github.com/k0sproject/k0s/pkg/autopilot/constant"
	apdel "github.com/k0sproject/k0s/pkg/autopilot/controller/delegate"
	appc "github.com/k0sproject/k0s/pkg/autopilot/controller/plans/core"
	k0sclientset "github.com/k0sproject/k0s/pkg/client/clientset"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/stretchr/testify/suite"
)

type apSignalErrorSuite struct {
	common.BootlooseSuite
}

// SetupSuite starts k0s once for the whole suite so that each test method
// can reuse the same running cluster without re-starting k0s.
func (s *apSignalErrorSuite) SetupSuite() {
	s.BootlooseSuite.SetupSuite()

	ctx, nodeName, require := s.Context(), s.ControllerNode(0), s.Require()

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

	// Wait for the ControlNode to be fully initialized with platform labels before
	// any test submits a plan, avoiding SignalMissingPlatform from the newplan handler.
	restConfig, err := s.GetKubeConfig(nodeName)
	require.NoError(err)
	k0sClient, err := k0sclientset.NewForConfig(restConfig)
	require.NoError(err)
	_, err = aptest.WaitForControlNodeReady(ctx, k0sClient, "controller0")
	require.NoError(err, "While waiting for ControlNode to have platform labels")
}

// TearDownTest deletes the plan after each test to ensure a clean state for
// the next test in the suite.
func (s *apSignalErrorSuite) TearDownTest() {
	ctx := s.Context()
	restConfig, err := s.GetKubeConfig(s.ControllerNode(0))
	if err != nil {
		return
	}
	client, err := k0sclientset.NewForConfig(restConfig)
	if err != nil {
		return
	}
	_ = client.AutopilotV1beta2().Plans().Delete(ctx, apconst.AutopilotName, metav1.DeleteOptions{})
}

// TestControllerDownloadFailure submits a plan with a valid URL but wrong sha256,
// verifies PlanApplyFailed is reached, and that the failure description is populated
// in both the plan status and the ControlNode's autopilot-last-error annotation.
func (s *apSignalErrorSuite) TestControllerDownloadFailure() {
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
            sha256: "0000000000000000000000000000000000000000000000000000000000000000"
          linux-arm64:
            url: http://localhost/dist/k0s
            sha256: "0000000000000000000000000000000000000000000000000000000000000000"
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

	client, err := k0sclientset.NewForConfig(restConfig)
	s.Require().NoError(err)

	plan, err := aptest.WaitForPlanState(ctx, client, apconst.AutopilotName, appc.PlanApplyFailed)
	s.Require().NoError(err, "While waiting for plan to fail")

	if s.Len(plan.Status.Commands, 1) {
		cmd := plan.Status.Commands[0]
		s.Equal(appc.PlanApplyFailed, cmd.State)
		s.NotNil(cmd.K0sUpdate)
		s.Contains(cmd.Description, "controller0", "command description should contain node name")
		s.Contains(cmd.Description, "FailedDownload", "command description should contain failure reason")

		if s.Len(cmd.K0sUpdate.Controllers, 1) {
			ctrl := cmd.K0sUpdate.Controllers[0]
			s.Equal(appc.SignalApplyFailed, ctrl.State)
			s.Contains(ctrl.Description, "FailedDownload", "node description should contain failure reason")
		}
	}

	signalErr, err := aptest.WaitForControlNodeSignalError(ctx, client, "controller0")
	s.Require().NoError(err, "While waiting for ControlNode signal error to be set")
	s.Equal("id123", signalErr.PlanID)
	s.Equal("FailedDownload", signalErr.Reason)
	s.NotEmpty(signalErr.Message)
}

// TestErrorClearedOnRetry verifies that a stale signal error is cleared when a
// new plan is submitted after a failure.
func (s *apSignalErrorSuite) TestErrorClearedOnRetry() {
	ctx := s.Context()

	restConfig, err := s.GetKubeConfig(s.ControllerNode(0))
	s.Require().NoError(err)

	client, err := k0sclientset.NewForConfig(restConfig)
	s.Require().NoError(err)

	// Phase 1: submit a plan with a bad sha256 to trigger a FailedDownload.
	badPlan := `
apiVersion: autopilot.k0sproject.io/v1beta2
kind: Plan
metadata:
  name: autopilot
spec:
  id: id-bad
  timestamp: now
  commands:
    - k0supdate:
        version: v0.0.0
        forceupdate: true
        platforms:
          linux-amd64:
            url: http://localhost/dist/k0s
            sha256: "0000000000000000000000000000000000000000000000000000000000000000"
          linux-arm64:
            url: http://localhost/dist/k0s
            sha256: "0000000000000000000000000000000000000000000000000000000000000000"
        targets:
          controllers:
            discovery:
              static:
                nodes:
                  - controller0
`
	_, err = common.Create(ctx, restConfig, []byte(badPlan))
	s.Require().NoError(err)

	_, err = aptest.WaitForPlanState(ctx, client, apconst.AutopilotName, appc.PlanApplyFailed)
	s.Require().NoError(err, "While waiting for plan to fail")

	_, err = aptest.WaitForControlNodeSignalError(ctx, client, "controller0")
	s.Require().NoError(err, "While waiting for ControlNode signal error to be set")

	// Phase 2: delete the failed plan.
	err = client.AutopilotV1beta2().Plans().Delete(ctx, apconst.AutopilotName, metav1.DeleteOptions{})
	s.Require().NoError(err)

	// Phase 3: submit a new plan with the correct URL and no sha256.
	goodPlan := `
apiVersion: autopilot.k0sproject.io/v1beta2
kind: Plan
metadata:
  name: autopilot
spec:
  id: id-good
  timestamp: now
  commands:
    - k0supdate:
        version: v0.0.0
        forceupdate: true
        platforms:
          linux-amd64:
            url: http://localhost/dist/k0s
          linux-arm64:
            url: http://localhost/dist/k0s
        targets:
          controllers:
            discovery:
              static:
                nodes:
                  - controller0
`
	_, err = common.Create(ctx, restConfig, []byte(goodPlan))
	s.Require().NoError(err)

	_, err = aptest.WaitForPlanState(ctx, client, apconst.AutopilotName, appc.PlanCompleted)
	s.Require().NoError(err, "While waiting for retry plan to complete")

	// Phase 4: verify the signal error annotation is cleared on the ControlNode.
	cn, err := client.AutopilotV1beta2().ControlNodes().Get(ctx, "controller0", metav1.GetOptions{})
	s.Require().NoError(err)
	_, hasError := cn.GetAnnotations()[apdel.SignalErrorAnnotation]
	s.False(hasError, "autopilot-last-error annotation should be cleared after a successful signal")
}

// TestAPSignalErrorSuite runs the signal error test suite using a single
// controller in --single mode.
func TestAPSignalErrorSuite(t *testing.T) {
	suite.Run(t, &apSignalErrorSuite{
		common.BootlooseSuite{
			K0sFullPath:     "/tmp/k0s",
			ControllerCount: 1,
			WorkerCount:     0,
			LaunchMode:      common.LaunchModeOpenRC,
		},
	})
}
