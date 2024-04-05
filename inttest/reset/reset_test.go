/*
Copyright 2024 k0s authors

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

package reset

import (
	"testing"

	testifysuite "github.com/stretchr/testify/suite"

	"github.com/k0sproject/k0s/inttest/common"
)

type suite struct {
	common.BootlooseSuite
}

func (s *suite) TestReset() {
	ctx := s.Context()
	workerNode := s.WorkerNode(0)

	if ok := s.Run("k0s gets up", func() {
		s.Require().NoError(s.InitController(0, "--disable-components=konnectivity-server,metrics-server"))
		s.Require().NoError(s.RunWorkers())

		kc, err := s.KubeClient(s.ControllerNode(0))
		s.Require().NoError(err)

		err = s.WaitForNodeReady(workerNode, kc)
		s.NoError(err)

		s.T().Log("waiting to see CNI pods ready")
		s.NoError(common.WaitForKubeRouterReady(ctx, kc), "CNI did not start")
	}); !ok {
		return
	}

	s.Run("k0s reset", func() {
		ssh, err := s.SSH(ctx, workerNode)
		s.Require().NoError(err)
		defer ssh.Disconnect()

		s.NoError(ssh.Exec(ctx, "test -d /var/lib/k0s", common.SSHStreams{}), "/var/lib/k0s is not a directory")
		s.NoError(ssh.Exec(ctx, "test -d /run/k0s", common.SSHStreams{}), "/run/k0s is not a directory")

		s.NoError(ssh.Exec(ctx, "pidof containerd-shim-runc-v2 >&2", common.SSHStreams{}), "Expected some running containerd shims")

		s.NoError(s.StopWorker(workerNode), "Failed to stop k0s")

		streams, flushStreams := common.TestLogStreams(s.T(), "reset")
		err = ssh.Exec(ctx, "k0s reset --debug", streams)
		flushStreams()
		s.NoError(err, "k0s reset didn't exit cleanly")

		// /var/lib/k0s is a mount point in the Docker container and can't be deleted, so it must be empty
		s.NoError(ssh.Exec(ctx, `x="$(ls -A /var/lib/k0s)" && echo "$x" >&2 && [ -z "$x" ]`, common.SSHStreams{}), "/var/lib/k0s is not empty")
		s.NoError(ssh.Exec(ctx, "! test -e /run/k0s", common.SSHStreams{}), "/run/k0s still exists")
		s.NoError(ssh.Exec(ctx, "! pidof containerd-shim-runc-v2 >&2", common.SSHStreams{}), "Expected no running containerd shims")
	})
}

func TestResetSuite(t *testing.T) {
	testifysuite.Run(t, &suite{
		common.BootlooseSuite{
			ControllerCount: 1,
			WorkerCount:     1,
		},
	})
}
