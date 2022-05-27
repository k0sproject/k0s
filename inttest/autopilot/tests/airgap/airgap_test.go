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
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	apitcomm "github.com/k0sproject/k0s/inttest/autopilot/common"
	apv1beta2 "github.com/k0sproject/k0s/pkg/apis/autopilot.k0sproject.io/v1beta2"
	apcomm "github.com/k0sproject/k0s/pkg/autopilot/common"
	apconst "github.com/k0sproject/k0s/pkg/autopilot/constant"
	appc "github.com/k0sproject/k0s/pkg/autopilot/controller/plans/core"

	"github.com/stretchr/testify/suite"
)

type airgapSuite struct {
	apitcomm.FootlooseSuite
}

const network = "airgap"

// SetupSuite creates the required network before starting footloose.
func (s *airgapSuite) SetupSuite() {
	s.Require().NoError(s.CreateNetwork(network))
	s.FootlooseSuite.SetupSuite()
}

// TearDownSuite tears down the network created after footloose has finished.
func (s *airgapSuite) TearDownSuite() {
	s.FootlooseSuite.TearDownSuite()
	s.Require().NoError(s.DestroyNetwork(network))
}

// SetupTest prepares the controller and filesystem, getting it into a consistent
// state which we can run tests against.
func (s *airgapSuite) SetupTest() {
	ipAddress := s.GetControllerIPAddress(0)

	// Note that the token is intentionally empty for the first controller
	s.Require().NoError(s.InitController(0, "--disable-components=metrics-server"))
	s.Require().NoError(s.WaitJoinAPI(s.ControllerNode(0)))

	// With k0s running, then start autopilot
	s.Require().NoError(s.InitControllerAutopilot(0, "--kubeconfig=/var/lib/k0s/pki/admin.conf", "--mode=controller"))

	cClient, err := s.ExtensionsClient(s.ControllerNode(0))
	s.Require().NoError(err)

	_, perr := apcomm.WaitForCRDByName(context.TODO(), cClient, "plans.autopilot.k0sproject.io", 2*time.Minute)
	s.Require().NoError(perr)
	_, cerr := apcomm.WaitForCRDByName(context.TODO(), cClient, "controlnodes.autopilot.k0sproject.io", 2*time.Minute)
	s.Require().NoError(cerr)

	// Collect an `admin.conf` from a controller for use with worker nodes, and add in the
	// first controller
	controllerAdminConfg := s.GetFileFromController(0, "/var/lib/k0s/pki/admin.conf")
	controllerAdminConfg = strings.Replace(controllerAdminConfg, "localhost", ipAddress, -1)

	// Create a worker join token
	workerJoinToken, err := s.GetJoinToken("worker")
	s.Require().NoError(err)

	// Start the workers using the join token
	s.Require().NoError(s.RunWorkersWithToken(workerJoinToken))

	wClient, err := s.KubeClient(s.ControllerNode(0))
	s.Require().NoError(err)

	s.Require().NoError(s.WaitForNodeReady(s.WorkerNode(0), wClient))

	// With k0s running, then start autopilot
	s.PutFile(s.WorkerNode(0), "/var/lib/k0s/admin.conf", controllerAdminConfg)
	s.Require().NoError(s.InitWorkerAutopilot(0, "--kubeconfig=/var/lib/k0s/admin.conf", "--mode=worker"))

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
        version: ` + apitcomm.TargetK0sVersion + `
        platforms:
          linux-amd64:
            url: ` + apitcomm.Versions[apitcomm.TargetK0sVersion]["linux-amd64"]["airgap"]["url"] + `
            sha256: ` + apitcomm.Versions[apitcomm.TargetK0sVersion]["linux-amd64"]["airgap"]["sha256"] + `
        workers:
          discovery:
            static:
              nodes:
                - worker0
    - k0supdate:
        version: ` + apitcomm.TargetK0sVersion + `
        platforms:
          linux-amd64:
            url: ` + apitcomm.Versions[apitcomm.TargetK0sVersion]["linux-amd64"]["k0s"]["url"] + `
            sha256: ` + apitcomm.Versions[apitcomm.TargetK0sVersion]["linux-amd64"]["k0s"]["sha256"] + `
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

	manifestFile := "/tmp/happy.yaml"
	s.PutFileTemplate(s.ControllerNode(0), manifestFile, planTemplate, nil)

	out, err := s.RunCommandController(0, fmt.Sprintf("/usr/local/bin/k0s kubectl apply -f %s", manifestFile))
	s.T().Logf("kubectl apply output: '%s'", out)
	s.Require().NoError(err)

	client, err := s.AutopilotClient(s.ControllerNode(0))
	s.NoError(err)
	s.NotEmpty(client)

	// The plan has enough information to perform a successful update of k0s, so wait for it.
	plan, err := apcomm.WaitForPlanByName(context.TODO(), client, apconst.AutopilotName, 10*time.Minute, func(obj interface{}) bool {
		if plan, ok := obj.(*apv1beta2.Plan); ok {
			return plan.Status.State == appc.PlanCompleted
		}

		return false
	})

	s.NoError(err)
	s.Equal(appc.PlanCompleted, plan.Status.State)

	// We are not confirming the image importing functionality of k0s, but we can get a pretty good idea if it worked.

	// Does the bundle exist on the worker, in the proper directory?
	lsout, err := s.RunCommandWorker(0, "ls /var/lib/k0s/images/k0s-airgap-bundle-*")
	s.NoError(err)
	s.NotEmpty(lsout)
}

// TestAirgapSuite sets up a suite using 3 controllers for quorum, and runs various
// autopilot upgrade scenarios against them.
func TestAirgapSuite(t *testing.T) {
	suite.Run(t, &airgapSuite{
		apitcomm.FootlooseSuite{
			ControllerCount:    1,
			WorkerCount:        1,
			ControllerNetworks: []string{network},
			WorkerNetworks:     []string{network},
		},
	})
}
