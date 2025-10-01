// SPDX-FileCopyrightText: 2022 k0s authors
// SPDX-License-Identifier: Apache-2.0

package kubeletcertrotate

import (
	"encoding/json"
	"fmt"
	"testing"
	"time"

	apv1beta2 "github.com/k0sproject/k0s/pkg/apis/autopilot/v1beta2"
	apconst "github.com/k0sproject/k0s/pkg/autopilot/constant"
	appc "github.com/k0sproject/k0s/pkg/autopilot/controller/plans/core"
	k0sclientset "github.com/k0sproject/k0s/pkg/client/clientset"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/k0sproject/k0s/inttest/common"
	aptest "github.com/k0sproject/k0s/inttest/common/autopilot"

	"github.com/stretchr/testify/suite"
)

type kubeletCertRotateSuite struct {
	common.BootlooseSuite
}

// SetupTest prepares the controller and filesystem, getting it into a consistent
// state which we can run tests against.
func (s *kubeletCertRotateSuite) SetupTest() {
	ctx := s.Context()
	s.Require().NoError(s.WaitForSSH(s.ControllerNode(0), 2*time.Minute, 1*time.Second))
	s.Require().NoError(s.InitController(0, "--disable-components=metrics-server", "--kube-controller-manager-extra-args='--cluster-signing-duration=3m'"))
	s.Require().NoError(s.WaitJoinAPI(s.ControllerNode(0)))

	extClient, err := s.ExtensionsClient(s.ControllerNode(0))
	s.Require().NoError(err)

	s.Require().NoError(aptest.WaitForCRDByName(ctx, extClient, "plans"))
	s.Require().NoError(aptest.WaitForCRDByName(ctx, extClient, "controlnodes"))

	// Create a worker join token
	workerJoinToken, err := s.GetJoinToken("worker")
	s.Require().NoError(err)

	// Start the workers using the join token
	s.Require().NoError(s.RunWorkersWithToken(workerJoinToken, "--kubelet-root-dir=/var/lib/kubelet"))

	client, err := s.KubeClient(s.ControllerNode(0))
	s.Require().NoError(err)

	for idx := range s.WorkerCount {
		s.Require().NoError(s.WaitForNodeReady(s.WorkerNode(idx), client))
	}

	// Knowing that `kube-controller-manager` is issuing certificates that live for
	// only 3m, if we can successfully apply autopilot plans AFTER kubelet key/certs have changed, we should
	// be able to confidentially say that the transport cert rotation is fine.
	workerSSH, err := s.SSH(s.Context(), s.WorkerNode(0))
	s.Require().NoError(err)
	s.T().Log("waiting to see kubelet rotating the client cert before triggering Plan creation")
	_, err = workerSSH.ExecWithOutput(s.Context(), "inotifywait --no-dereference /var/lib/kubelet/pki/kubelet-client-current.pem")
	s.Require().NoError(err)
	output, err := workerSSH.ExecWithOutput(s.Context(), "k0s status -ojson")
	s.Require().NoError(err)
	var status map[string]any
	s.Require().NoError(json.Unmarshal([]byte(output), &status))
	success, found, err := unstructured.NestedBool(status, "WorkerToAPIConnectionStatus", "Success")
	s.Require().NoError(err)
	s.Require().True(found)
	s.Require().True(success)
	s.TestApply()
}

func (s *kubeletCertRotateSuite) applyPlan(id string) {
	ctx := s.Context()

	restConfig, err := s.GetKubeConfig(s.ControllerNode(0))
	s.Require().NoError(err)

	// Ensure that a plan and yaml do not exist (safely)
	_, err = s.RunCommandController(0, "/usr/local/bin/k0s kubectl delete plan autopilot | true")
	s.Require().NoError(err)

	planTemplate := `
apiVersion: autopilot.k0sproject.io/v1beta2
kind: Plan
metadata:
  name: autopilot
spec:
  id: ` + id + `
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
          workers:
            discovery:
              static:
                nodes:
                  - worker0
`

	// Create the plan

	_, err = common.Create(ctx, restConfig, []byte(planTemplate))
	s.Require().NoError(err)
	s.T().Logf("Plan created")

	// The plan has enough information to perform a successful update of k0s, so wait for it.
	client, err := k0sclientset.NewForConfig(restConfig)
	s.Require().NoError(err)
	plan, err := aptest.WaitForPlanState(ctx, client, apconst.AutopilotName, appc.PlanCompleted)
	s.Require().NoError(err)

	// Ensure all state/status are completed
	if s.Len(plan.Status.Commands, 1) {
		cmd := plan.Status.Commands[0]

		s.Equal(appc.PlanCompleted, cmd.State)
		s.NotNil(cmd.K0sUpdate)
		s.NotNil(cmd.K0sUpdate.Workers)

		for _, group := range [][]apv1beta2.PlanCommandTargetStatus{cmd.K0sUpdate.Controllers, cmd.K0sUpdate.Workers} {
			for _, node := range group {
				s.Equal(appc.SignalCompleted, node.State)
			}
		}
	}
}

// TestApply applies a well-formed `plan` yaml, and asserts that
// all of the correct values across different objects + controllers are correct.
func (s *kubeletCertRotateSuite) TestApply() {
	// TODO: There is a bug that prevents plans from being applied more than once
	// unless you clear the autopilot metadata from the controlnode/node.
	//
	// Leaving this as 1 for now until the issue is fixed.

	for i := range 1 {
		s.T().Logf("Applying autopilot plan #%d", i)
		s.applyPlan(fmt.Sprintf("id%d", i))
	}
}

// TestKubeletCertRotateSuite sets up a suite using 3 controllers for quorum, and runs various
// autopilot upgrade scenarios against them.
func TestKubeletCertRotateSuite(t *testing.T) {
	suite.Run(t, &kubeletCertRotateSuite{
		common.BootlooseSuite{
			ControllerCount: 1,
			WorkerCount:     1,
			LaunchMode:      common.LaunchModeOpenRC,
		},
	})
}
