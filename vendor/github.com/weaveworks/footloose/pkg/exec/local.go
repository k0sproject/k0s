/*
Copyright 2018 The Kubernetes Authors.

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

package exec

import (
	"fmt"
	"io"
	"os"
	osexec "os/exec"
	"strings"

	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
)

// LocalCmd wraps os/exec.Cmd, implementing the kind/pkg/exec.Cmd interface
type LocalCmd struct {
	*osexec.Cmd
}

var _ Cmd = &LocalCmd{}

// LocalCmder is a factory for LocalCmd, implementing Cmder
type LocalCmder struct{}

var _ Cmder = &LocalCmder{}

// Command returns a new exec.Cmd backed by Cmd
func (c *LocalCmder) Command(name string, arg ...string) Cmd {
	return &LocalCmd{
		Cmd: osexec.Command(name, arg...),
	}
}

// SetEnv sets env
func (cmd *LocalCmd) SetEnv(env ...string) {
	cmd.Env = env
}

// SetStdin sets stdin
func (cmd *LocalCmd) SetStdin(r io.Reader) {
	cmd.Stdin = r
}

// SetStdout set stdout
func (cmd *LocalCmd) SetStdout(w io.Writer) {
	cmd.Stdout = w
}

// SetStderr sets stderr
func (cmd *LocalCmd) SetStderr(w io.Writer) {
	cmd.Stderr = w
}

// Run runs
func (cmd *LocalCmd) Run() error {
	log.Debugf("Running: %v %v", cmd.Path, cmd.Args)
	return cmd.Cmd.Run()
}

func ExecuteCommand(command string, args ...string) (string, error) {
	cmd := osexec.Command(command, args...)
	out, err := cmd.CombinedOutput()
	cmdArgs := strings.Join(cmd.Args, " ")
	//log.Debugf("Command %q returned %q\n", cmdArgs, out)
	if err != nil {
		return "", errors.Wrapf(err, "command %q exited with %q", cmdArgs, out)
	}

	// TODO: strings.Builder?
	return strings.TrimSpace(string(out)), nil
}

func ExecForeground(command string, args ...string) (int, error) {
	cmd := osexec.Command(command, args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err := cmd.Run()
	cmdArgs := strings.Join(cmd.Args, " ")

	var cmdErr error
	var exitCode int

	if err != nil {
		cmdErr = fmt.Errorf("external command %q exited with an error: %v", cmdArgs, err)

		if exitError, ok := err.(*osexec.ExitError); ok {
			exitCode = exitError.ExitCode()
		} else {
			cmdErr = fmt.Errorf("failed to get exit code for external command %q", cmdArgs)
		}
	}

	return exitCode, cmdErr
}
