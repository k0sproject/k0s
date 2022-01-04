/*
Copyright 2022 k0s authors

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
package common

import "fmt"

// GetFile gets file from the controller with given index
func (s *FootlooseSuite) GetFileFromController(controllerIdx int, path string) string {
	sshCon, err := s.SSH(s.ControllerNode(controllerIdx))
	s.Require().NoError(err)
	defer sshCon.Disconnect()
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

// AppendFile appends content to file on given node
func (s *FootlooseSuite) AppendFile(node, path, content string) {
	ssh, err := s.SSH(node)
	s.Require().NoError(err)
	defer ssh.Disconnect()
	// TODO: send data via pipe instead, so we can write data with single quotes '
	_, err = ssh.ExecWithOutput(fmt.Sprintf("echo '%s' >> %s", content, path))

	s.Require().NoError(err)
}

// Mkdir makes directory
func (s *FootlooseSuite) MakeDir(node, path string) {
	ssh, err := s.SSH(node)
	s.Require().NoError(err)
	defer ssh.Disconnect()
	_, err = ssh.ExecWithOutput(fmt.Sprintf("mkdir %s", path))
	s.Require().NoError(err)
}
