package common

import "fmt"

// GetFile gets file from the controller with given index
func (s *FootlooseSuite) GetFileFromController(controllerIdx int, path string) string {
	sshCon, err := s.SSH(s.ControllerNode(controllerIdx))
	s.Require().NoError(err)
	content, err := sshCon.ExecWithOutput(fmt.Sprintf("cat %s", path))
	s.Require().NoError(err)

	return content
}

// PutFile writes content to file on given node
func (s *FootlooseSuite) PutFile(node, path, content string) {
	ssh, err := s.SSH(node)
	s.Require().NoError(err)
	defer ssh.Disconnect()
	// TODO: send data via pipe instead, so we can write data with single quotes '
	_, err = ssh.ExecWithOutput(fmt.Sprintf("echo '%s' >%s", content, path))

	s.Require().NoError(err)
}
