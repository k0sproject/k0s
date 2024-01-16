// Copyright 2022 k0s authors
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

package airgap

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/k0sproject/k0s/pkg/airgap"
	"github.com/k0sproject/k0s/pkg/apis/k0s/v1beta1"
	apconst "github.com/k0sproject/k0s/pkg/autopilot/constant"
	appc "github.com/k0sproject/k0s/pkg/autopilot/controller/plans/core"
	"github.com/k0sproject/k0s/pkg/constant"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"

	"github.com/k0sproject/k0s/inttest/common"
	aptest "github.com/k0sproject/k0s/inttest/common/autopilot"

	"github.com/stretchr/testify/suite"
)

type airgapSuite struct {
	common.BootlooseSuite
}

// SetupTest prepares the controller and filesystem, getting it into a consistent
// state which we can run tests against.
func (s *airgapSuite) SetupTest() {
	ctx := s.Context()

	// Note that the token is intentionally empty for the first controller
	s.Require().NoError(s.InitController(0, "--disable-components=metrics-server"))
	s.Require().NoError(s.WaitJoinAPI(s.ControllerNode(0)))

	cClient, err := s.ExtensionsClient(s.ControllerNode(0))
	s.Require().NoError(err)

	s.Require().NoError(aptest.WaitForCRDByName(ctx, cClient, "plans"))
	s.Require().NoError(aptest.WaitForCRDByName(ctx, cClient, "controlnodes"))

	wClient, err := s.KubeClient(s.ControllerNode(0))
	s.Require().NoError(err)

	// Create a worker join token
	workerJoinToken, err := s.GetJoinToken("worker")
	s.Require().NoError(err)

	// Start the workers using the join token
	s.Require().NoError(s.RunWorkersWithToken(workerJoinToken))

	s.Require().NoError(s.WaitForNodeReady(s.WorkerNode(0), wClient))

	// Wait until all the cluster components are up.
	s.Require().NoError(common.WaitForKubeRouterReady(ctx, wClient), "While waiting for kube-router to become ready")
	s.Require().NoError(common.WaitForCoreDNSReady(ctx, wClient), "While waiting for CoreDNS to become ready")
	s.Require().NoError(common.WaitForPodLogs(ctx, wClient, "kube-system"), "While waiting for some pod logs")

	// Check that none of the images in the airgap bundle are pinned.
	// This will happen as soon as k0s imports them after the Autopilot update.
	ssh, err := s.SSH(ctx, s.WorkerNode(0))
	s.Require().NoError(err)
	defer ssh.Disconnect()
	for _, i := range airgap.GetImageURIs(v1beta1.DefaultClusterSpec(), true) {
		if strings.HasPrefix(i, constant.KubePauseContainerImage+":") {
			continue // The pause image is pinned by containerd itself
		}
		output, err := ssh.ExecWithOutput(ctx, fmt.Sprintf(`k0s ctr i ls "name==%s"`, i))
		if s.NoError(err, "Failed to check %s", i) {
			s.NotContains(output, "io.cri-containerd.pinned=pinned", "%s is already pinned", i)
		}
	}
}

func (s *airgapSuite) TestApply() {
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
                - worker0
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

	ctx := s.Context()

	// The container images have already been pulled by the cluster.
	// Airgapping is kind of cosmetic here.
	err := (&common.Airgap{
		SSH:  s.SSH,
		Logf: s.T().Logf,
	}).LockdownMachines(ctx,
		s.ControllerNode(0), s.WorkerNode(0),
	)
	s.Require().NoError(err)

	manifestFile := "/tmp/happy.yaml"
	s.PutFileTemplate(s.ControllerNode(0), manifestFile, planTemplate, nil)

	updateStart := time.Now()

	out, err := s.RunCommandController(0, fmt.Sprintf("/usr/local/bin/k0s kubectl apply -f %s", manifestFile))
	s.T().Logf("kubectl apply output: '%s'", out)
	s.Require().NoError(err)

	client, err := s.AutopilotClient(s.ControllerNode(0))
	s.Require().NoError(err)
	s.NotEmpty(client)

	// The plan has enough information to perform a successful update of k0s, so wait for it.
	_, err = aptest.WaitForPlanState(ctx, client, apconst.AutopilotName, appc.PlanCompleted)
	s.Require().NoError(err)

	// Does the bundle exist on the worker, in the proper directory?
	lsout, err := s.RunCommandWorker(0, "ls /var/lib/k0s/images/bundle.tar")
	s.NoError(err)
	s.NotEmpty(lsout)

	// Wait until all the cluster components are up.
	kc, err := s.KubeClient(s.ControllerNode(0))
	s.Require().NoError(err)
	s.Require().NoError(common.WaitForKubeRouterReady(ctx, kc), "While waiting for kube-router to become ready")
	s.Require().NoError(common.WaitForCoreDNSReady(ctx, kc), "While waiting for CoreDNS to become ready")
	s.Require().NoError(common.WaitForPodLogs(ctx, kc, "kube-system"), "While waiting for some pod logs")

	// At that moment we can assume that all pods have at least started.
	// Inspect the Pulled events if there are some unexpected image pulls.
	events, err := kc.CoreV1().Events("").List(ctx, metav1.ListOptions{
		FieldSelector: fields.AndSelectors(
			fields.OneTermEqualSelector("involvedObject.kind", "Pod"),
			fields.OneTermEqualSelector("reason", "Pulled"),
		).String(),
	})
	s.Require().NoError(err)

	for _, event := range events.Items {
		if event.LastTimestamp.After(updateStart) {
			if !strings.HasSuffix(event.Message, "already present on machine") {
				s.Fail("Unexpected Pulled event", event.Message)
			} else {
				s.T().Log("Observed Pulled event:", event.Message)
			}
		}
	}

	// Check that all the images in the airgap bundle have been pinned by k0s.
	// This proves that k0s has processed the image bundle.
	ssh, err := s.SSH(ctx, s.WorkerNode(0))
	s.Require().NoError(err)
	defer ssh.Disconnect()
	for _, i := range airgap.GetImageURIs(v1beta1.DefaultClusterSpec(), true) {
		output, err := ssh.ExecWithOutput(ctx, fmt.Sprintf(`k0s ctr i ls "name==%s"`, i))
		if s.NoError(err, "Failed to check %s", i) {
			s.Contains(output, "io.cri-containerd.pinned=pinned", "%s is not pinned", i)
		}
	}
}

// TestAirgapSuite sets up a suite using 3 controllers for quorum, and runs various
// autopilot upgrade scenarios against them.
func TestAirgapSuite(t *testing.T) {
	suite.Run(t, &airgapSuite{
		common.BootlooseSuite{
			ControllerCount: 1,
			WorkerCount:     1,
			LaunchMode:      common.LaunchModeOpenRC,

			AirgapImageBundleMountPoints: []string{"/dist/bundle.tar"},
		},
	})
}
