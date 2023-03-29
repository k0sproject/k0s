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

package cleanup

import (
	"errors"
	"fmt"
	"io/fs"
	"os/exec"
	"strings"

	"github.com/k0sproject/k0s/pkg/install"
)

type services struct {
	Config *Config
}

// Name returns the name of the step
func (s *services) Name() string {
	return "uninstall service step"
}

// Run uninstalls k0s services that are found on the host
func (s *services) Run() error {
	var msg []string

	for _, role := range []string{"controller", "worker"} {
		if err := install.UninstallService(role); err != nil && !(errors.Is(err, fs.ErrNotExist) || isExitCode(err, 1)) {
			msg = append(msg, err.Error())
		}
	}
	if len(msg) > 0 {
		return fmt.Errorf("%v", strings.Join(msg, "\n"))
	}
	return nil
}

func isExitCode(err error, exitcode int) bool {
	var e *exec.ExitError
	return errors.As(err, &e) && e.ProcessState.ExitCode() == exitcode
}
