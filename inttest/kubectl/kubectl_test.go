package kubectl

import (
	"testing"

	"github.com/k0sproject/k0s/inttest/common"
	"github.com/stretchr/testify/suite"
)

type KubectlSuite struct {
	common.FootlooseSuite
}

const pluginContent = `#!/bin/bash

echo "foo-plugin"
`

func (s *KubectlSuite) TestEmbeddedKubectl() {
	s.Require().NoError(s.InitController(0))
	s.PutFile(s.ControllerNode(0), "/bin/kubectl-foo", pluginContent)

	ssh, err := s.SSH(s.ControllerNode(0))
	s.Require().NoError(err)
	defer ssh.Disconnect()

	_, err = ssh.ExecWithOutput("chmod +x /bin/kubectl-foo")
	s.Require().NoError(err)
	_, err = ssh.ExecWithOutput("ln -s /usr/bin/k0s /usr/bin/kubectl")
	s.Require().NoError(err)

	s.T().Log("Check that different ways to call kubectl subcommand work")

	tests := []struct {
		Name    string
		Command string
		Check   func(output string, err error)
	}{
		{
			Name:    "full subcommand name",
			Command: "k0s kubectl version",
			Check: func(output string, e error) {
				s.Require().NoError(e)
				s.Require().Contains(output, "Client Version: version.Info")
			},
		},
		{
			Name:    "short subcommand name",
			Command: "k0s kc version",
			Check: func(output string, e error) {
				s.Require().NoError(e)
				s.Require().Contains(output, "Client Version: version.Info")
			},
		},
		{
			Name:    "full command arguments",
			Command: "k0s kubectl version -v 8",
			Check: func(output string, e error) {
				s.Require().NoError(e)
				// Check for debug log messages
				s.Require().Contains(output, "round_trippers.go")
			},
		},
		{
			Name:    "short command arguments",
			Command: "k0s kc version -v 8",
			Check: func(output string, e error) {
				s.Require().NoError(e)
				// Check for debug log messages
				s.Require().Contains(output, "round_trippers.go")
			},
		},
		{
			Name:    "full command plugin loader",
			Command: "k0s kubectl foo",
			Check: func(output string, e error) {
				s.Require().NoError(e)
				s.Require().Equal("foo-plugin", output, "Unexpected output: %v", output)
			},
		},
		{
			Name:    "short command plugin loader",
			Command: "k0s kc foo",
			Check: func(output string, e error) {
				s.Require().NoError(e)
				s.Require().Equal("foo-plugin", output, "Unexpected output: %v", output)
			},
		},

		{
			Name:    "symlink command",
			Command: "kubectl version",
			Check: func(output string, e error) {
				s.Require().NoError(e)
				s.Require().Contains(output, "Client Version: version.Info")
			},
		},
		{
			Name:    "symlink arguments",
			Command: "kubectl version -v 8",
			Check: func(output string, e error) {
				s.Require().NoError(e)
				// Check for debug log messages
				s.Require().Contains(output, "round_trippers.go")
			},
		},
		{
			Name:    "symlink plugin loader",
			Command: "k0s kubectl foo",
			Check: func(output string, e error) {
				s.Require().NoError(e)
				s.Require().Equal("foo-plugin", output, "Unexpected output: %v", output)
			},
		},
	}
	for _, test := range tests {
		s.T().Logf("Trying %s with command `%s`", test.Name, test.Command)
		output, err := ssh.ExecWithOutput(test.Command)
		test.Check(output, err)
	}
}

func TestKubectlCommand(t *testing.T) {
	suite.Run(t, &KubectlSuite{
		common.FootlooseSuite{
			ControllerCount: 1,
		},
	})
}
