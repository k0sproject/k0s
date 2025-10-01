// SPDX-FileCopyrightText: 2020 k0s authors
// SPDX-License-Identifier: Apache-2.0

package common

import (
	"bytes"
	"fmt"
	"io"
	"path/filepath"

	"github.com/k0sproject/k0s/internal/pkg/templatewriter"
)

// GetFile gets file from the controller with given index
func (s *BootlooseSuite) GetFileFromController(controllerIdx int, path string) string {
	sshCon, err := s.SSH(s.Context(), s.ControllerNode(controllerIdx))
	s.Require().NoError(err)
	defer sshCon.Disconnect()
	content, err := sshCon.ExecWithOutput(s.Context(), "cat "+path)
	s.Require().NoError(err)

	return content
}

// WriteFile writes the data provided by reader to a file at the given path on
// the given node.
func (s *BootlooseSuite) WriteFile(node, path string, reader io.Reader) {
	ssh, err := s.SSH(s.Context(), node)
	s.Require().NoError(err)
	defer ssh.Disconnect()
	s.Require().NoError(ssh.Exec(s.Context(), "cat >"+path, SSHStreams{In: reader}))
}

// WriteFileContent writes content to a file at the given path on the given
// node.
func (s *BootlooseSuite) WriteFileContent(node, path string, content []byte) {
	s.WriteFile(node, path, bytes.NewReader(content))
}

// PutFile writes content to file on given node
func (s *BootlooseSuite) PutFile(node, path, content string) {
	s.WriteFileContent(node, path, []byte(content))
}

// PutFileTemplate writes content to file on given node using templated data
func (s *BootlooseSuite) PutFileTemplate(node string, filename string, template string, data any) {
	tw := templatewriter.TemplateWriter{
		Name:     filepath.Base(filename),
		Template: template,
		Data:     data,
		Path:     filename,
	}

	var buf bytes.Buffer
	s.Require().NoError(tw.WriteToBuffer(&buf))
	s.WriteFile(node, filename, &buf)
}

// Mkdir makes directory
func (s *BootlooseSuite) MakeDir(node, path string) {
	ssh, err := s.SSH(s.Context(), node)
	s.Require().NoError(err)
	defer ssh.Disconnect()
	s.Require().NoError(ssh.Exec(s.Context(), fmt.Sprintf("mkdir -p -- %q", path), SSHStreams{}))
}
