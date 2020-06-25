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

	cmd     *exec.Cmd
	running bool
}

// Supervise Starts supervising the given process
func (s *Supervisor) Supervise() {
	s.running = true
	go func() {
		log := logrus.WithField("component", s.Name)
		log.Info("Starting to supervise")
		for {
			s.cmd = exec.Command(s.BinPath, s.Args...)

			s.cmd.Env = getEnv()

			// TODO Wire up the stdout&stderr to somehow through logger to be able to distinguis the components.
			s.cmd.Stdout = os.Stdout
			s.cmd.Stderr = os.Stderr

			err := s.cmd.Start()
			if err != nil {
				log.Warnf("Failed to start: %s", err)
			} else {
				log.Info("Started succesfully, go nuts")
			}
			err = s.cmd.Wait()
			log.Warnf("Process exited with code: %d", s.cmd.ProcessState.ExitCode())
			if !s.running {
				return
			}
			// TODO Maybe some backoff thingy would be nice
			log.Info("respawning in 5 secs")
			time.Sleep(5 * time.Second)
		}
	}()
}

// Stop stops the supervised
func (s *Supervisor) Stop() error {
	s.running = false
	err := s.cmd.Process.Signal(syscall.SIGTERM)
	err = s.cmd.Wait()
	return err
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
