// SPDX-FileCopyrightText: 2021 k0s authors
// SPDX-License-Identifier: Apache-2.0

package ctr

import (
	"regexp"
	"strings"
	"testing"

	"github.com/k0sproject/k0s/inttest/common"
	"github.com/stretchr/testify/suite"
)

type CtrSuite struct {
	common.BootlooseSuite
}

func (s *CtrSuite) TestK0sCtrCommand() {
	ssh, err := s.SSH(s.Context(), s.ControllerNode(0))
	s.Require().NoError(err)
	defer ssh.Disconnect()

	_, err = ssh.ExecWithOutput(s.Context(), "/usr/local/bin/k0s install controller --enable-worker")
	s.Require().NoError(err)

	_, err = ssh.ExecWithOutput(s.Context(), "/usr/local/bin/k0s start")
	s.Require().NoError(err)

	err = s.WaitForKubeAPI(s.ControllerNode(0))
	s.Require().NoError(err)

	kc, err := s.KubeClient(s.ControllerNode(0))
	s.Require().NoError(err)

	err = s.WaitForNodeReady(s.ControllerNode(0), kc)
	s.NoError(err)

	output, err := ssh.ExecWithOutput(s.Context(), "/usr/local/bin/k0s ctr namespaces list 2>/dev/null")
	s.Require().NoError(err)

	flatOutput := removeRedundantSpaces(output)
	errMsg := "returned output of command 'k0s ctr namespaces list' is different than expected: " + output
	s.Equal("NAME LABELS k8s.io", flatOutput, errMsg)

	output, err = ssh.ExecWithOutput(s.Context(), "/usr/local/bin/k0s ctr version")
	s.Require().NoError(err)
	s.Require().NotContains(output, "WARNING")
}

func TestCtrCommandSuite(t *testing.T) {
	s := CtrSuite{
		common.BootlooseSuite{
			ControllerCount: 1,
		},
	}
	suite.Run(t, &s)
}

func removeRedundantSpaces(output string) string {
	pattern := regexp.MustCompile(`\s+`)
	out := pattern.ReplaceAllString(output, " ")
	out = strings.TrimSpace(out)
	return out
}
