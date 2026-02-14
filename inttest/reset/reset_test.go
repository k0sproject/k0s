// SPDX-FileCopyrightText: 2024 k0s authors
// SPDX-License-Identifier: Apache-2.0

package reset

import (
	"bytes"
	_ "embed"
	"fmt"
	"io"
	"strings"
	"testing"

	testifysuite "github.com/stretchr/testify/suite"

	"github.com/k0sproject/k0s/inttest/common"
)

type suite struct {
	common.BootlooseSuite
}

//go:embed clutter-data-dir.sh
var clutterScript []byte

func (s *suite) TestReset() {
	ctx := s.Context()
	workerNode := s.WorkerNode(0)

	if !s.Run("k0s gets up", func() {
		s.Require().NoError(s.InitController(0, "--disable-components=konnectivity-server,metrics-server"))
		s.Require().NoError(s.RunWorkers("--kubelet-root-dir=/var/lib/kubelet"))

		kc, err := s.KubeClient(s.ControllerNode(0))
		s.Require().NoError(err)

		err = s.WaitForNodeReady(workerNode, kc)
		s.NoError(err)

		s.T().Log("waiting to see CNI pods ready")
		s.NoError(common.WaitForKubeRouterReady(ctx, kc), "CNI did not start")

		ssh, err := s.SSH(ctx, workerNode)
		s.Require().NoError(err)
		defer ssh.Disconnect()

		s.NoError(ssh.Exec(ctx, "test -d /var/lib/k0s", common.SSHStreams{}), "/var/lib/k0s is not a directory")
		s.NoError(ssh.Exec(ctx, "test -d /run/k0s", common.SSHStreams{}), "/run/k0s is not a directory")
		s.NoError(ssh.Exec(ctx, "test -d /var/lib/kubelet", common.SSHStreams{}), "/var/lib/kubelet is not a directory")

		s.NoError(ssh.Exec(ctx, "pidof containerd-shim-runc-v2 >&2", common.SSHStreams{}), "Expected some running containerd shims")
	}) {
		return
	}

	var clutteringPaths bytes.Buffer

	if !s.Run("prepare k0s reset", func() {
		s.NoError(s.StopWorker(workerNode), "Failed to stop k0s")

		ssh, err := s.SSH(ctx, workerNode)
		s.Require().NoError(err)
		defer ssh.Disconnect()

		streams, flushStreams := common.TestLogStreams(s.T(), "clutter data dir")
		streams.In = bytes.NewReader(clutterScript)
		streams.Out = io.MultiWriter(&clutteringPaths, streams.Out)
		err = ssh.Exec(ctx, "sh -s -- /var/lib/k0s", streams)
		flushStreams()
		s.Require().NoError(err)
	}) {
		return
	}

	s.Run("k0s reset", func() {
		ssh, err := s.SSH(ctx, workerNode)
		s.Require().NoError(err)
		defer ssh.Disconnect()

		streams, flushStreams := common.TestLogStreams(s.T(), "reset")
		err = ssh.Exec(ctx, "k0s reset --debug --kubelet-root-dir=/var/lib/kubelet", streams)
		flushStreams()
		s.NoError(err, "k0s reset didn't exit cleanly")

		for path := range strings.SplitSeq(string(bytes.TrimSpace(clutteringPaths.Bytes())), "\n") {
			if strings.HasPrefix(path, "/var/lib/k0s") {
				s.NoError(ssh.Exec(ctx, fmt.Sprintf("! test -e %q", path), common.SSHStreams{}), "Failed to verify non-existence of %s", path)
			} else {
				s.NoError(ssh.Exec(ctx, fmt.Sprintf("test -e %q", path), common.SSHStreams{}), "Failed to verify existence of %s", path)
			}
		}

		// /var/lib/k0s is a mount point in the Docker container and can't be deleted, so it must be empty
		s.NoError(ssh.Exec(ctx, `x="$(ls -A /var/lib/k0s)" && echo "$x" >&2 && [ -z "$x" ]`, common.SSHStreams{}), "/var/lib/k0s is not empty")
		s.NoError(ssh.Exec(ctx, "! test -e /var/lib/kubelet", common.SSHStreams{}), "/var/lib/kubelet still exists")
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
