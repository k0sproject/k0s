package supervisor

import (
	"fmt"
	"os"
	"os/exec"
	"path"
	"strings"
	"syscall"
	"time"

	"github.com/Mirantis/mke/pkg/constant"
	"github.com/sirupsen/logrus"
)

// Supervisor is dead simple and stupid process supervisor, just tries to keep the process running in a while-true loop
type Supervisor struct {
	Name    string
	BinPath string
	Args    []string
	Dir     string

	cmd  *exec.Cmd
	quit chan bool
	done chan bool
}

// Supervise Starts supervising the given process
func (s *Supervisor) Supervise() {
	s.quit = make(chan bool)
	s.done = make(chan bool)
	go func() {
		log := logrus.WithField("component", s.Name)
		log.Info("Starting to supervise")
		defer func() {
			s.done <- true
		}()
		for {
			s.cmd = exec.Command(s.BinPath, s.Args...)
			s.cmd.Dir = s.Dir

			s.cmd.Env = getEnv()

			// detach from the process group so children don't
			// get signals sent directly to parent.
			s.cmd.SysProcAttr = &syscall.SysProcAttr{
				Setpgid: true,
				Pgid:    0,
			}

			// TODO Wire up the stdout&stderr to somehow through logger to be able to distinguis the components.
			s.cmd.Stdout = os.Stdout
			s.cmd.Stderr = os.Stderr

			err := s.cmd.Start()
			if err != nil {
				log.Warnf("Failed to start: %s", err)
			} else {
				log.Info("Started succesfully, go nuts")
			}
			waitresult := make(chan error)
			go func() {
				waitresult <- s.cmd.Wait()
			}()

			select {
			case <-s.quit:
				log.Infof("Shutting down pid %d", s.cmd.Process.Pid)
				err = s.cmd.Process.Signal(syscall.SIGTERM)
				if err != nil {
					log.Warnf("Failed to send SIGTERM to pid %d: %s", s.cmd.Process.Pid, err)
				} else {
					err = <-waitresult
				}
				return
			case err = <-waitresult:
				log.Warnf("Process exited with code: %d", s.cmd.ProcessState.ExitCode())

			}

			// TODO Maybe some backoff thingy would be nice
			log.Info("respawning in 5 secs")

			select {
			case <-s.quit:
				log.Debug("respawn cancelled")
				return
			case <-time.After(5 * time.Second):
				log.Debug("respawning")
			}
		}
	}()
}

// Stop stops the supervised
func (s *Supervisor) Stop() error {
	s.quit <- true
	<-s.done
	return nil
}

// Modifies the current processes env so that we inject mke embedded bins into path
func getEnv() []string {
	env := os.Environ()
	for i, e := range env {
		if strings.HasPrefix(e, "PATH=") {
			env[i] = fmt.Sprintf("PATH=%s:%s", os.Getenv("PATH"), path.Join(constant.DataDir, "bin"))
		}
	}
	return env
}
