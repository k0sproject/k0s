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
