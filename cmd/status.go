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

package cmd

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"runtime"
	"strings"

	"github.com/k0sproject/k0s/pkg/build"
	"github.com/k0sproject/k0s/pkg/install"
	ps "github.com/mitchellh/go-ps"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v2"
)

var (
	output string

	status    *K0sStatus
	statusCmd = &cobra.Command{
		Use:     "status",
		Short:   "Helper command for get general information about k0s",
		Example: `The command will return information about system init, PID, k0s role, kubeconfig and similar.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			var err error

			if status, err = getPid(); err != nil {
				return err
			}
			if status.Role, err = getRole(status.Pid); err != nil {
				return err
			}
			if status.SysInit, status.StubFile, err = getSysInit(status.Role); err != nil {
				return err
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
	s.Version = build.Version
	switch s.output {
	case "json":
		jsn, _ := json.MarshalIndent(s, "", "   ")
		fmt.Println(string(jsn))
	case "yaml":
		ym, _ := yaml.Marshal(s)
		fmt.Println(string(ym))
	default:
		fmt.Println("Version:", s.Version)
		if s.Pid == 0 {
			fmt.Println("K0s not running")
		} else {
			fmt.Println("Process ID:", s.Pid)
			fmt.Println("Parent Process ID:", s.PPid)
			fmt.Println("Role:", s.Role)
		}

		fmt.Println("Init System:", s.SysInit)
		if s.StubFile != "" {
			fmt.Println("Service file:", s.StubFile)
		}
	}

}
func getSysInit(role string) (sysInit string, stubFile string, err error) {
	if role == "server+worker" {
		role = "server"
	}
	if sysInit, err = install.GetSysInit(); err != nil {
		return sysInit, stubFile, err
	}
	if sysInit == "linux-systemd" {
		stubFile = fmt.Sprintf("/etc/systemd/system/k0s%s.service", role)
		if _, err := os.Stat(stubFile); err != nil {
			stubFile = ""
		}
	} else if sysInit == "linux-openrc" {
		stubFile = fmt.Sprintf("/etc/init.d/k0s%s", role)
		if _, err := os.Stat(stubFile); err != nil {
			stubFile = ""
		}
	}

	return sysInit, stubFile, err

}
func getRole(pid int) (role string, err error) {
	if runtime.GOOS == "windows" {
		return "worker", nil
	}

	var raw []byte
	if raw, err = ioutil.ReadFile(fmt.Sprintf("/proc/%d/cmdline", pid)); err != nil {
		return "", fmt.Errorf("K0s not running")
	}
	cmdln := string(raw)
	if strings.Contains(cmdln, "enable-worker") {
		return "server+worker", nil
	} else if strings.Contains(cmdln, "server") {
		return "server", nil
	}
	return role, nil
}

func getPid() (status *K0sStatus, err error) {
	processList, err := ps.Processes()
	if err != nil {

		return nil, err
	}
	for _, p := range processList {
		if p.Executable() == "k0s" && hasChildern(p.Pid(), processList) {
			status = &K0sStatus{Pid: p.Pid(),
				PPid: p.PPid()}

			return status, nil
		}
	}

	return &K0sStatus{}, nil
}

func hasChildern(pid int, processes []ps.Process) bool {
	for _, p := range processes {
		if p.PPid() == pid {
			return true
		}
	}
	return false
}
