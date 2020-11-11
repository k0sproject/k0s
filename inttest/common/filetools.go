package common

import "fmt"

// GetFile gets file from the controller with given index
func (s *FootlooseSuite) GetFileFromController(controllerIdx int, path string) string {
	node := fmt.Sprintf("controller%d", controllerIdx)
	sshCon, err := s.SSH(node)
	s.Require().NoError(err)
	content, err := sshCon.ExecWithOutput(fmt.Sprintf("cat %s", path))
	s.Require().NoError(err)

	return content
}
