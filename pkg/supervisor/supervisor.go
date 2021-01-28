/*
Copyright 2020 Mirantis, Inc.

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
package supervisor

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/k0sproject/k0s/internal/util"
	"github.com/k0sproject/k0s/pkg/constant"
)

// Supervisor is dead simple and stupid process supervisor, just tries to keep the process running in a while-true loop
type Supervisor struct {
	Name           string
	BinPath        string
	RunDir         string
	DataDir        string
	Args           []string
	PidFile        string
	UID            int
	GID            int
	TimeoutStop    time.Duration
	TimeoutRespawn time.Duration

	cmd  *exec.Cmd
	quit chan bool
	done chan bool
	log  *logrus.Entry
}

// processWaitQuit waits for a process to exit or a shut down signal
// returns true if shutdown is requested
func (s *Supervisor) processWaitQuit() bool {
	waitresult := make(chan error)
	go func() {
		waitresult <- s.cmd.Wait()
	}()

	pidbuf := []byte(strconv.Itoa(s.cmd.Process.Pid) + "\n")
	err := ioutil.WriteFile(s.PidFile, pidbuf, constant.PidFileMode)
	if err != nil {
		s.log.Warnf("Failed to write file %s: %v", s.PidFile, err)
	}
	defer os.Remove(s.PidFile)

	select {
	case <-s.quit:
		for {
			s.log.Infof("Shutting down pid %d", s.cmd.Process.Pid)
			err := s.cmd.Process.Signal(syscall.SIGTERM)
			if err != nil {
				s.log.Warnf("Failed to send SIGTERM to pid %d: %s", s.cmd.Process.Pid, err)
			}
			select {
			case <-time.After(s.TimeoutStop):
				continue
			case <-waitresult:
				return true
			}
		}
	case err := <-waitresult:
		if err != nil {
			s.log.Warn(err)
		} else {
			s.log.Warnf("Process exited with code: %d", s.cmd.ProcessState.ExitCode())
		}
	}
	return false
}

// Supervise Starts supervising the given process
func (s *Supervisor) Supervise() error {
	s.log = logrus.WithField("component", s.Name)
	s.PidFile = path.Join(s.RunDir, s.Name) + ".pid"
	if err := util.InitDirectory(s.RunDir, constant.RunDirMode); err != nil {
		s.log.Warnf("failed to initialize dir: %v", err)
		return err
	}

	if s.TimeoutStop == 0 {
		s.TimeoutStop = 5 * time.Second
	}
	if s.TimeoutRespawn == 0 {
		s.TimeoutRespawn = 5 * time.Second
	}

	started := make(chan error)
	go func() {
		s.log.Info("Starting to supervise")
		for {
			s.cmd = exec.Command(s.BinPath, s.Args...)
			s.cmd.Dir = s.DataDir
			s.cmd.Env = getEnv(s.DataDir)

			// detach from the process group so children don't
			// get signals sent directly to parent.
			s.cmd.SysProcAttr = DetachAttr(s.UID, s.GID)

			s.cmd.Stdout = s.log.Writer()
			s.cmd.Stderr = s.log.Writer()

			err := s.cmd.Start()
			if err != nil {
				s.log.Warnf("Failed to start: %s", err)
				if s.quit == nil {
					started <- err
					return
				}
			} else {
				if s.quit == nil {
					s.log.Info("Started successfully, go nuts")
					s.quit = make(chan bool)
					s.done = make(chan bool)
					defer func() {
						s.done <- true
					}()
					started <- nil
				} else {
					s.log.Info("Restarted")
				}
				if s.processWaitQuit() {
					return
				}
			}

			// TODO Maybe some backoff thingy would be nice
			s.log.Infof("respawning in %s", s.TimeoutRespawn.String())

			select {
			case <-s.quit:
				s.log.Debug("respawn cancelled")
				return
			case <-time.After(s.TimeoutRespawn):
				s.log.Debug("respawning")
			}
		}
	}()
	return <-started
}

// Stop stops the supervised
func (s *Supervisor) Stop() error {
	if s.quit != nil {
		s.quit <- true
		<-s.done
	}
	return nil
}

// Modifies the current processes env so that we inject k0s embedded bins into path
func getEnv(dataDir string) []string {
	env := os.Environ()
	for i, e := range env {
		if strings.HasPrefix(e, "PATH=") {
			env[i] = fmt.Sprintf("PATH=%s:%s", os.Getenv("PATH"), path.Join(dataDir, "bin"))
		}
	}
	return env
}
