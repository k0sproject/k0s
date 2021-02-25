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

package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"

	"github.com/sirupsen/logrus"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v2"

	"github.com/k0sproject/k0s/pkg/install"
)

var (
	output string

	status    *K0sStatus
	statusCmd = &cobra.Command{
		Use:     "status",
		Short:   "Helper command for get general information about k0s",
		Example: `The command will return information about system init, PID, k0s role, kubeconfig and similar.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if runtime.GOOS == "windows" {
				return fmt.Errorf("currently not supported on windows")
			}
			var err error
			if status, err = getPid(); err != nil {
				return err
			}

			if status.Pid != 0 {
				ver, err := getK0sVersion(status.Pid)
				if err != nil {
					return err
				}
				status.Version = ver

				if os.Geteuid() != 0 {
					logrus.Fatal("k0s status must be run as root!")
				}

				if status.SysInit, status.StubFile, err = install.GetSysInit(status.Role); err != nil {
					return err
				}
				if status.Role, err = install.GetRoleByPID(status.Pid); err != nil {
					return err
				}
			} else {
				fmt.Fprintln(os.Stderr, "K0s not running")
				os.Exit(1)
			}

			status.output = output
			status.String()
			return nil
		},
	}
)

func init() {
	status = &K0sStatus{}
	statusCmd.PersistentFlags().StringVarP(&output, "out", "o", "", "sets type of out put to json or yaml")
}

type K0sStatus struct {
	Version  string
	Pid      int
	PPid     int
	Role     string
	SysInit  string
	StubFile string
	output   string
}

func (s K0sStatus) String() {
	switch s.output {
	case "json":
		jsn, _ := json.MarshalIndent(s, "", "   ")
		fmt.Println(string(jsn))
	case "yaml":
		ym, _ := yaml.Marshal(s)
		fmt.Println(string(ym))
	default:
		if s.Pid == 0 {
			fmt.Println("K0s not running")
			return
		}

		fmt.Println("Version:", s.Version)
		fmt.Println("Process ID:", s.Pid)
		fmt.Println("Parent Process ID:", s.PPid)
		fmt.Println("Role:", s.Role)

		if s.SysInit != "" {
			fmt.Println("Init System:", s.SysInit)

		}
		if s.StubFile != "" {
			fmt.Println("Service file:", s.StubFile)
		}
	}

}

func getPid() (status *K0sStatus, err error) {
	pid, ppid, err := install.GetProcessID()
	if err == nil && pid != nil {
		status = &K0sStatus{
			Pid:  *pid,
			PPid: *ppid,
		}
		return status, nil
	}
	return &K0sStatus{}, nil
}

func getK0sVersion(pid int) (string, error) {
	cmd := fmt.Sprintf("/proc/%d/exe", pid)
	stdout, err := exec.Command(cmd, "version").Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSuffix(string(stdout), "\n"), nil
}
