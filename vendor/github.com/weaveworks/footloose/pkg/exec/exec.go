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

// Package exec contains an interface for executing commands, along with helpers
// TODO(bentheelder): add standardized timeout functionality & a default timeout
// so that commands cannot hang indefinitely (!)
package exec

import (
	"bufio"
	"bytes"
	"io"
	"os"

	log "github.com/sirupsen/logrus"
)

// Cmd abstracts over running a command somewhere, this is useful for testing
type Cmd interface {
	Run() error
	// Each entry should be of the form "key=value"
	SetEnv(...string)
	SetStdin(io.Reader)
	SetStdout(io.Writer)
	SetStderr(io.Writer)
}

// Cmder abstracts over creating commands
type Cmder interface {
	// command, args..., just like os/exec.Cmd
	Command(string, ...string) Cmd
}

// DefaultCmder is a LocalCmder instance used for convienience, packages
// originally using os/exec.Command can instead use pkg/kind/exec.Command
// which forwards to this instance
// TODO(bentheelder): swap this for testing
// TODO(bentheelder): consider not using a global for this :^)
var DefaultCmder = &LocalCmder{}

// Command is a convience wrapper over DefaultCmder.Command
func Command(command string, args ...string) Cmd {
	return DefaultCmder.Command(command, args...)
}

// CommandWithLogging is a convience wrapper over Command
// display any errors received by the executed command
func CommandWithLogging(command string, args ...string) error {
	cmd := Command(command, args...)
	output, err := CombinedOutputLines(cmd)
	if err != nil {
		// log error output if there was any
		for _, line := range output {
			log.Error(line)
		}
	}
	return err

}

// CombinedOutputLines is like os/exec's cmd.CombinedOutput(),
// but over our Cmd interface, and instead of returning the byte buffer of
// stderr + stdout, it scans these for lines and returns a slice of output lines
func CombinedOutputLines(cmd Cmd) (lines []string, err error) {
	var buff bytes.Buffer
	cmd.SetStdout(&buff)
	cmd.SetStderr(&buff)
	err = cmd.Run()
	scanner := bufio.NewScanner(&buff)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	return lines, err
}

// InheritOutput sets cmd's output to write to the current process's stdout and stderr
func InheritOutput(cmd Cmd) {
	cmd.SetStderr(os.Stderr)
	cmd.SetStdout(os.Stdout)
}

// RunLoggingOutputOnFail runs the cmd, logging error output if Run returns an error
func RunLoggingOutputOnFail(cmd Cmd) error {
	var buff bytes.Buffer
	cmd.SetStdout(&buff)
	cmd.SetStderr(&buff)
	err := cmd.Run()
	if err != nil {
		log.Error("failed with:")
		scanner := bufio.NewScanner(&buff)
		for scanner.Scan() {
			log.Error(scanner.Text())
		}
	}
	return err
}
