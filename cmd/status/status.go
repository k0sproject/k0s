/*
Copyright 2021 k0s authors

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
package status

import (
	"fmt"
	"os"
	"runtime"
	"strings"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/k0sproject/k0s/pkg/config"
	"github.com/k0sproject/k0s/pkg/install"
)

type CmdOpts config.CLIOptions

var (
	output string
	s      *install.K0sStatus
)

func NewStatusCmd() *cobra.Command {
	s = &install.K0sStatus{}
	cmd := &cobra.Command{
		Use:     "status",
		Short:   "Helper command for get general information about k0s",
		Example: `The command will return information about system init, PID, k0s role, kubeconfig and similar.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if runtime.GOOS == "windows" {
				return fmt.Errorf("currently not supported on windows")
			}
			var err error
			if s, err = install.GetPid(); err != nil {
				return err
			}

			if s.Pid != 0 {
				ver, err := s.GetK0sVersion()
				if err != nil {
					return err
				}
				s.Version = ver

				if os.Geteuid() != 0 {
					logrus.Fatal("k0s status must be run as root!")
				}

				if s.Role, err = install.GetRoleByPID(s.Pid); err != nil {
					return err
				}

				if s.SysInit, s.StubFile, err = install.GetSysInit(strings.TrimSuffix(s.Role, "+worker")); err != nil {
					return err
				}
			} else {
				fmt.Fprintln(os.Stderr, "K0s not running")
				os.Exit(1)
			}
			s.Output = output
			s.String()
			return nil
		},
	}
	cmd.SilenceUsage = true
	cmd.PersistentFlags().StringVarP(&output, "out", "o", "", "sets type of output to json or yaml")
	return cmd
}
