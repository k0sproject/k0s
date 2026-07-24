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

	"github.com/k0sproject/bootloose/pkg/config"
	"github.com/k0sproject/k0s/inttest/common"
)

const dataDir = "/var/lib/k0s"

type suite struct {
	common.BootlooseSuite

	// kubeletRootDir is the directory passed to the worker and to k0s reset
	// via --kubelet-root-dir.
	kubeletRootDir string
}

//go:embed clutter-data-dir.sh
var clutterScript []byte

// kubeletRootDirSeparate reports whether the kubelet root dir lives outside the
// data dir. A separate kubelet root dir is its own mount point that reset must
// keep in place (only emptying it); one under the data dir gets removed
// together with the data dir.
func (s *suite) kubeletRootDirSeparate() bool {
	return !strings.HasPrefix(s.kubeletRootDir, dataDir+"/")
}

func (s *suite) TestReset() {
	ctx := s.Context()
	workerNode := s.WorkerNode(0)
	separate := s.kubeletRootDirSeparate()

	// A mount nested under the kubelet root dir, and the source it's bound
	// from. Reset must unmount and remove the nested mount, but leave the
	// (out-of-tree) bind source alone.
	nestedMount := s.kubeletRootDir + "/k0s_reset_inttest_nested_mount"
	nestedMountSource := "/k0s_reset_inttest_nested_source"

	if !s.Run("k0s gets up", func() {
		s.Require().NoError(s.InitController(0, "--disable-components=konnectivity-server,metrics-server"))
		s.Require().NoError(s.RunWorkers("--kubelet-root-dir=" + s.kubeletRootDir))

		kc, err := s.KubeClient(s.ControllerNode(0))
		s.Require().NoError(err)

		err = s.WaitForNodeReady(workerNode, kc)
		s.NoError(err)

		s.T().Log("waiting to see CNI pods ready")
		s.NoError(common.WaitForKubeRouterReady(ctx, kc), "CNI did not start")

		ssh, err := s.SSH(ctx, workerNode)
		s.Require().NoError(err)
		defer ssh.Disconnect()

		s.NoError(ssh.Exec(ctx, "test -d "+dataDir, common.SSHStreams{}), "%s is not a directory", dataDir)
		s.NoError(ssh.Exec(ctx, "test -d /run/k0s", common.SSHStreams{}), "/run/k0s is not a directory")
		s.NoError(ssh.Exec(ctx, "test -d "+s.kubeletRootDir, common.SSHStreams{}), "%s is not a directory", s.kubeletRootDir)

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

		// Clutter the data dir with files and (recursive/over-) mounts that
		// reset has to clean up.
		streams, flushStreams := common.TestLogStreams(s.T(), "clutter data dir")
		streams.In = bytes.NewReader(clutterScript)
		streams.Out = io.MultiWriter(&clutteringPaths, streams.Out)
		err = ssh.Exec(ctx, "sh -s -- "+dataDir, streams)
		flushStreams()
		s.Require().NoError(err)

		// The worker's kubelet already bind-mounts its root dir, so the kubelet
		// root dir is itself a (busy) mount point at this point. Add another
		// mount nested under it, so reset has to unmount and remove the nested
		// mount while coping with the kubelet root dir being a busy mount point:
		// leaving it in place when it's separate from the data dir, and
		// unmounting it when it lives under the data dir.
		setup := strings.Join([]string{
			fmt.Sprintf("mkdir -p %s %s", nestedMount, nestedMountSource),
			fmt.Sprintf("mount --bind %s %s", nestedMountSource, nestedMount),
		}, " && ")
		s.Require().NoError(ssh.Exec(ctx, setup, common.SSHStreams{}), "failed to set up kubelet root dir mounts")
	}) {
		return
	}

	s.Run("k0s reset", func() {
		ssh, err := s.SSH(ctx, workerNode)
		s.Require().NoError(err)
		defer ssh.Disconnect()

		streams, flushStreams := common.TestLogStreams(s.T(), "reset")
		err = ssh.Exec(ctx, "k0s reset --debug --kubelet-root-dir="+s.kubeletRootDir, streams)
		flushStreams()
		s.NoError(err, "k0s reset didn't exit cleanly")

		// Everything cluttered under the data dir must be gone (mounts
		// unmounted, contents removed); the bind-mount sources live outside the
		// data dir and must still be there.
		for path := range strings.SplitSeq(string(bytes.TrimSpace(clutteringPaths.Bytes())), "\n") {
			if strings.HasPrefix(path, dataDir) {
				s.NoError(ssh.Exec(ctx, fmt.Sprintf("! test -e %q", path), common.SSHStreams{}), "Failed to verify non-existence of %s", path)
			} else {
				s.NoError(ssh.Exec(ctx, fmt.Sprintf("test -e %q", path), common.SSHStreams{}), "Failed to verify existence of %s", path)
			}
		}

		// A mount nested under the kubelet root dir must be unmounted and
		// removed, while its out-of-tree bind source is left untouched.
		s.NoError(ssh.Exec(ctx, "! test -e "+nestedMount, common.SSHStreams{}), "%s still exists", nestedMount)
		s.NoError(ssh.Exec(ctx, "test -d "+nestedMountSource, common.SSHStreams{}), "%s should have survived reset", nestedMountSource)

		// The data dir is a mount point in the Docker container and can't be
		// deleted, so it must be emptied.
		s.NoError(ssh.Exec(ctx, fmt.Sprintf(`x="$(ls -A %s)" && echo "$x" >&2 && [ -z "$x" ]`, dataDir), common.SSHStreams{}), "%s is not empty", dataDir)

		if separate {
			// A kubelet root dir outside the data dir is its own mount point:
			// reset can't delete it, so it must survive but be emptied.
			s.NoError(ssh.Exec(ctx, fmt.Sprintf(`x="$(ls -A %s)" && echo "$x" >&2 && [ -z "$x" ]`, s.kubeletRootDir), common.SSHStreams{}), "%s is not empty", s.kubeletRootDir)
		} else {
			// A kubelet root dir under the data dir is removed along with it.
			s.NoError(ssh.Exec(ctx, "! test -e "+s.kubeletRootDir, common.SSHStreams{}), "%s still exists", s.kubeletRootDir)
		}

		s.NoError(ssh.Exec(ctx, "! test -e /run/k0s", common.SSHStreams{}), "/run/k0s still exists")
		s.NoError(ssh.Exec(ctx, "! pidof containerd-shim-runc-v2 >&2", common.SSHStreams{}), "Expected no running containerd shims")
	})
}

// TestResetSuite covers a kubelet root dir on its own mount, separate from the
// data dir: reset must leave the mount point in place and only empty it.
func TestResetSuite(t *testing.T) {
	testifysuite.Run(t, &suite{
		BootlooseSuite: common.BootlooseSuite{
			ControllerCount: 1,
			WorkerCount:     1,
			ExtraVolumes: []config.Volume{
				{
					Type:        "volume",
					Destination: "/var/lib/kubelet",
				},
			},
		},
		kubeletRootDir: "/var/lib/kubelet",
	})
}

// TestResetKubeletRootDirUnderDataDirSuite covers a kubelet root dir under the
// data dir (its default location): reset must unmount and remove it together
// with the data dir.
func TestResetKubeletRootDirUnderDataDirSuite(t *testing.T) {
	testifysuite.Run(t, &suite{
		BootlooseSuite: common.BootlooseSuite{
			ControllerCount: 1,
			WorkerCount:     1,
		},
		kubeletRootDir: dataDir + "/kubelet",
	})
}
