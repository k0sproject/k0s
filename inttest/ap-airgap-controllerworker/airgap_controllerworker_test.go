// SPDX-FileCopyrightText: 2026 k0s authors
// SPDX-License-Identifier: Apache-2.0

package airgapcontrollerworker

import (
	"testing"

	apconst "github.com/k0sproject/k0s/pkg/autopilot/constant"
	appc "github.com/k0sproject/k0s/pkg/autopilot/controller/plans/core"
	k0sclientset "github.com/k0sproject/k0s/pkg/client/clientset"

	"github.com/k0sproject/k0s/inttest/common"
	aptest "github.com/k0sproject/k0s/inttest/common/autopilot"

	"github.com/stretchr/testify/suite"
)

// airgapControllerWorkerSuite verifies that autopilot airgap updates complete successfully on
// controller+worker nodes (regression test for a bug where the plan would get stuck in
// schedulablewait because the autopilot worker component is not started on controller+worker
// nodes, so the airgap signal on the v1.Node object is never processed).
type airgapControllerWorkerSuite struct {
	common.BootlooseSuite
}

// SetupTest prepares the controller+worker node and waits for it to be ready.
func (s *airgapControllerWorkerSuite) SetupTest() {
	ctx := s.Context()

	s.Require().NoError(s.InitController(0, "--enable-worker", "--disable-components=metrics-server"))
	s.Require().NoError(s.WaitJoinAPI(s.ControllerNode(0)))

	kc, err := s.KubeClient(s.ControllerNode(0))
	s.Require().NoError(err)

	s.Require().NoError(s.WaitForNodeReady(s.ControllerNode(0), kc))

	cClient, err := s.ExtensionsClient(s.ControllerNode(0))
	s.Require().NoError(err)

	s.Require().NoError(aptest.WaitForCRDByName(ctx, cClient, "plans"))
	s.Require().NoError(aptest.WaitForCRDByName(ctx, cClient, "controlnodes"))
}

// TestAirgapUpdateCompletesOnControllerWorkerNode verifies that an airgap update plan targeting a
// controller+worker node reaches PlanCompleted state.
//
// Regression test: a change that disabled the autopilot worker component on controller+worker
// nodes caused the airgap update to get stuck in schedulablewait because the worker component was
// the only consumer of the airgap signal on the Node object, and it was never started on
// controller+worker nodes.
func (s *airgapControllerWorkerSuite) TestAirgapUpdateCompletesOnControllerWorkerNode() {
	ctx := s.Context()

	planTemplate := `
apiVersion: autopilot.k0sproject.io/v1beta2
kind: Plan
metadata:
  name: autopilot
spec:
  id: id123
  timestamp: now
  commands:
    - airgapupdate:
        version: v0.0.0
        platforms:
          linux-amd64:
            url: http://localhost/dist/bundle.tar
          linux-arm64:
            url: http://localhost/dist/bundle.tar
        workers:
          discovery:
            static:
              nodes:
                - controller0
`

	restConfig, err := s.GetKubeConfig(s.ControllerNode(0))
	s.Require().NoError(err)

	_, err = common.Create(ctx, restConfig, []byte(planTemplate))
	s.Require().NoError(err)
	s.T().Log("Plan created")

	// The plan must reach PlanCompleted. If the controller+worker node is incorrectly left in the
	// workers list without anything to process its airgap signal, the plan will get stuck in
	// schedulablewait and the test will time out.
	client, err := k0sclientset.NewForConfig(restConfig)
	s.Require().NoError(err)
	_, err = aptest.WaitForPlanState(ctx, client, apconst.AutopilotName, appc.PlanCompleted)
	s.Require().NoError(err, "Airgap update plan did not complete: likely stuck in schedulablewait because the controller+worker node's airgap signal was not processed")

	// Verify the airgap bundle was placed on the controller+worker node.
	lsout, err := s.RunCommandController(0, "ls /var/lib/k0s/images/bundle.tar")
	s.NoError(err)
	s.NotEmpty(lsout)
}

// TestAirgapControllerWorkerSuite runs the airgap update suite against a single controller+worker
// node to expose the schedulablewait regression.
func TestAirgapControllerWorkerSuite(t *testing.T) {
	suite.Run(t, &airgapControllerWorkerSuite{
		common.BootlooseSuite{
			ControllerCount: 1,
			WorkerCount:     0,
			LaunchMode:      common.LaunchModeOpenRC,

			AirgapImageBundleMountPoints: []string{"/dist/bundle.tar"},
		},
	})
}
