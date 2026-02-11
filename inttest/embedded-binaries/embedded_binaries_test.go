// SPDX-FileCopyrightText: 2022 k0s authors
// SPDX-License-Identifier: Apache-2.0

package binaries

import (
	"path"
	"testing"

	"github.com/k0sproject/k0s/inttest/common"
	"github.com/stretchr/testify/suite"
)

type EmbeddedBinariesSuite struct {
	common.BootlooseSuite
}

func (s *EmbeddedBinariesSuite) TestK0sGetsUp() {
	s.NoError(s.InitController(0, "--enable-worker"))
	s.Require().NoError(s.WaitForKubeAPI(s.ControllerNode(0)))
	s.NoError(s.InitController(1, "--single"))
	s.Require().NoError(s.WaitForKubeAPI(s.ControllerNode(1)))

	kcC0, err := s.KubeClient(s.ControllerNode(0))
	s.Require().NoError(err)
	s.Require().NoError(s.WaitForNodeReady(s.ControllerNode(0), kcC0))
	kcC1, err := s.KubeClient(s.ControllerNode(1))
	s.Require().NoError(err)
	s.Require().NoError(s.WaitForNodeReady(s.ControllerNode(1), kcC1))

	sshC0, err := s.SSH(s.Context(), s.ControllerNode(0))
	s.Require().NoError(err)
	defer sshC0.Disconnect()

	sshC1, err := s.SSH(s.Context(), s.ControllerNode(1))
	s.Require().NoError(err)
	defer sshC1.Disconnect()

	s.Run("controller0", func() {
		var testCases = []struct {
			cmd        string
			checkError bool
			contains   string
		}{
			{"containerd -v", true, "containerd github.com/containerd/containerd"},
			{"containerd-shim-runc-v2 -v", true, "containerd-shim-runc-v2:"},
			{"etcd --version", true, ""},
			{"kube-apiserver --version", true, ""},
			{"kube-controller-manager --version", true, ""},
			{"kube-scheduler --version", true, ""},
			{"kubelet --version", true, ""},
			{"runc -v", true, ""},
			{"xtables-legacy-multi iptables -V", true, ""},
			{"xtables-nft-multi iptables -V", true, ""},
		}

		for _, tc := range testCases {
			s.Run(tc.cmd, func() {
				out, err := sshC0.ExecWithOutput(s.Context(), path.Join("/var/lib/k0s/bin", tc.cmd))
				if tc.checkError {
					s.Require().NoError(err, tc.cmd, out)
				}
				if tc.contains != "" {
					s.Require().Contains(out, tc.contains)
				}
			})
		}
	})

	s.Run("controller1", func() {
		var testCases = []struct {
			cmd        string
			checkError bool
			contains   string
		}{
			{"kine -v", true, ""},
		}

		for _, tc := range testCases {
			s.Run("", func() {
				out, err := sshC1.ExecWithOutput(s.Context(), path.Join("/var/lib/k0s/bin", tc.cmd))
				if tc.checkError {
					s.Require().NoError(err, tc.cmd, out)
				}
				if tc.contains != "" {
					s.Require().Contains(out, tc.contains)
				}
			})
		}
	})
}

func TestEmbeddedBinariesSuite(t *testing.T) {
	s := EmbeddedBinariesSuite{
		common.BootlooseSuite{
			ControllerCount: 2,
		},
	}
	suite.Run(t, &s)
}
