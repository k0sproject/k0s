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

package ha3x3

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	apconst "github.com/k0sproject/k0s/pkg/autopilot/constant"
	appc "github.com/k0sproject/k0s/pkg/autopilot/controller/plans/core"

	"github.com/k0sproject/k0s/inttest/common"
	aptest "github.com/k0sproject/k0s/inttest/common/autopilot"

	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type ha3x3Suite struct {
	common.BootlooseSuite
	k0sUpdateVersion string
}

const haControllerConfig = `
spec:
  api:
    externalAddress: %s
`

// SetupTest prepares the controller and filesystem, getting it into a consistent
// state which we can run tests against.
func (s *ha3x3Suite) SetupTest() {
	ctx := s.Context()
	ipAddress := s.GetLBAddress()
	var joinToken string

	for idx := 0; idx < s.BootlooseSuite.ControllerCount; idx++ {
		s.Require().NoError(s.WaitForSSH(s.ControllerNode(idx), 2*time.Minute, 1*time.Second))
		s.PutFile(s.ControllerNode(idx), "/tmp/k0s.yaml", fmt.Sprintf(haControllerConfig, ipAddress))

		// Note that the token is intentionally empty for the first controller
		s.Require().NoError(s.InitController(idx, "--config=/tmp/k0s.yaml", "--disable-components=metrics-server", joinToken))
		s.Require().NoError(s.WaitJoinAPI(s.ControllerNode(idx)))

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

	// Create a worker join token
	workerJoinToken, err := s.GetJoinToken("worker")
	s.Require().NoError(err)

	// Start the workers using the join token
	s.Require().NoError(s.RunWorkersWithToken(workerJoinToken))

	client, err := s.KubeClient(s.ControllerNode(0))
	s.Require().NoError(err)

	for idx := 0; idx < s.BootlooseSuite.WorkerCount; idx++ {
		s.Require().NoError(s.WaitForNodeReady(s.WorkerNode(idx), client))
	}
}

// TestApply applies a well-formed `plan` yaml, and asserts that
// all of the correct values across different objects + controllers are correct.
func (s *ha3x3Suite) TestApply() {
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
        version: ` + s.k0sUpdateVersion + `
        platforms:
          linux-amd64:
            url: http://localhost/dist/k0s-new
        targets:
          controllers:
            discovery:
              static:
                nodes:
                  - controller0
                  - controller1
                  - controller2
          workers:
            discovery:
              static:
                nodes:
                  - worker0
                  - worker1
                  - worker2
`

	ctx := s.Context()

	sshController, err := s.SSH(ctx, s.ControllerNode(0))
	s.Require().NoError(err)
	defer sshController.Disconnect()

	if version, err := s.GetK0sVersion(s.ControllerNode(0)); s.NoError(err, "Failed to get the base k0s version") {
		hasOldStack := version != s.k0sUpdateVersion && (strings.HasPrefix(version, "v1.27.") || strings.HasPrefix(version, "v1.28."))
		s.T().Logf("Base k0s version: %q, has old stack: %v", version, hasOldStack)
		s.checkKubeletConfigStackResources(ctx, sshController, hasOldStack)
	}

	sshWorker, err := s.SSH(ctx, s.WorkerNode(0))
	s.Require().NoError(err)
	defer sshWorker.Disconnect()

	iptablesModeBeforeUpdate, err := getIPTablesMode(ctx, sshWorker)
	if !s.NoError(err) {
		iptablesModeBeforeUpdate = ""
	}

	var createPlanOutput bytes.Buffer
	err = sshController.Exec(ctx, "k0s kc create -f -", common.SSHStreams{
		In:  strings.NewReader(planTemplate),
		Out: &createPlanOutput,
	})
	s.Require().NoError(err)
	s.T().Log(strings.TrimSpace(createPlanOutput.String()))

	client, err := s.AutopilotClient(s.ControllerNode(0))
	s.Require().NoError(err)

	// The plan has enough information to perform a successful update of k0s, so wait for it.
	s.T().Log("Waiting for autopilot plan to complete")
	plan, err := aptest.WaitForPlanState(ctx, client, apconst.AutopilotName, appc.PlanCompleted)
	s.Require().NoError(err)
	s.T().Log("Autopilot plan completed")

	// Ensure all state/status are completed
	if s.Len(plan.Status.Commands, 1) {
		cmd := plan.Status.Commands[0]
		s.Equal(appc.PlanCompleted, cmd.State)
		s.NotNil(cmd.K0sUpdate)
		s.NotNil(cmd.K0sUpdate.Controllers)
		s.NotNil(cmd.K0sUpdate.Workers)
		s.Equal(appc.PlanCompleted, cmd.State)
		s.NotNil(cmd.K0sUpdate)
		s.NotNil(cmd.K0sUpdate.Controllers)
		s.NotNil(cmd.K0sUpdate.Workers)

		if s.NotNil(cmd.K0sUpdate) {
			s.Len(cmd.K0sUpdate.Controllers, s.ControllerCount)
			for idx, controller := range cmd.K0sUpdate.Controllers {
				s.Equal(appc.SignalCompleted, controller.State, "For controller %d", idx)
			}

			s.Len(cmd.K0sUpdate.Workers, s.WorkerCount)
			for idx, worker := range cmd.K0sUpdate.Workers {
				s.Equal(appc.SignalCompleted, worker.State, "For worker %d", idx)
			}
		}
	}

	if version, err := s.GetK0sVersion(s.ControllerNode(0)); s.NoError(err) {
		s.Equal(s.k0sUpdateVersion, version)
	}

	if iptablesModeAfterUpdate, err := getIPTablesMode(ctx, sshWorker); s.NoError(err) {
		s.Equal(iptablesModeBeforeUpdate, iptablesModeAfterUpdate)
	}

	for idx := 0; idx < s.ControllerCount; idx++ {
		func() {
			ssh, err := s.SSH(ctx, s.ControllerNode(idx))
			s.Require().NoError(err)
			defer ssh.Disconnect()
			s.checkKubeletConfigComponentFolders(ctx, ssh)
		}()
	}

	s.checkKubeletConfigStackResources(ctx, sshController, false)
}

func (s *ha3x3Suite) checkKubeletConfigComponentFolders(ctx context.Context, ssh *common.SSHConnection) {
	var foundFiles bytes.Buffer
	if !s.NoError(
		ssh.Exec(ctx, "cd /var/lib/k0s/manifests/kubelet && find . -type f -print0", common.SSHStreams{Out: &foundFiles}),
		"Failed to list kubelet manifest folder",
	) {
		return
	}

	files := strings.Split(strings.TrimSuffix(foundFiles.String(), "\x00"), "\x00")

	// Check that removed.txt is present
	if idx := slices.Index(files, "./removed.txt"); idx < 0 {
		s.Failf("No removed.txt in kubelet manifests folder", "%v", files)
	} else {
		files = slices.Delete(files, idx, idx+1)
	}

	// Check that all other files are only disabled yaml files.
	for _, file := range files {
		match, err := filepath.Match("./kubelet-config.yaml.*.removed", file)
		s.Require().NoError(err)
		if !match {
			s.Failf("Unknown file in kubelet manifest folder", "%s in %v", file, files)
		}
	}
}

func (s *ha3x3Suite) checkKubeletConfigStackResources(ctx context.Context, ssh *common.SSHConnection, exist bool) {
	var out bytes.Buffer
	err := ssh.Exec(ctx, "k0s kc get configmaps,roles,rolebindings -A -l 'k0s.k0sproject.io/stack=kubelet' -oname", common.SSHStreams{Out: &out})

	if s.NoError(err) {
		if exist {
			s.NotEmpty(out.String())
		} else {
			s.Empty(out.String())
		}
	}
}

func getIPTablesMode(ctx context.Context, ssh *common.SSHConnection) (string, error) {
	var out bytes.Buffer
	err := ssh.Exec(ctx, "/var/lib/k0s/bin/iptables-save -V", common.SSHStreams{Out: &out})
	if err != nil {
		return "", err
	}

	version := out.String()
	if parts := strings.Split(version, " "); len(parts) == 3 {
		return parts[2], nil
	}

	return "", fmt.Errorf("expected something like %q, got %q", "iptables-save v1.8.9 (nf_tables)", version)
}

// TestHA3x3Suite sets up a suite using 3 controllers for quorum, and runs various
// autopilot upgrade scenarios against them.
func TestHA3x3Suite(t *testing.T) {
	k0sUpdateVersion := os.Getenv("K0S_UPDATE_TO_VERSION")
	require.NotEmpty(t, k0sUpdateVersion, "env var not set or empty: K0S_UPDATE_TO_VERSION")

	suite.Run(t, &ha3x3Suite{
		common.BootlooseSuite{
			ControllerCount: 3,
			WorkerCount:     3,
			WithLB:          true,
			LaunchMode:      common.LaunchModeOpenRC,
		},
		k0sUpdateVersion,
	})
}
