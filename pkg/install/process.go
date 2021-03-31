/*
Copyright 2021 k0s Authors

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
package install

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os/exec"
	"runtime"
	"strings"

	"github.com/mitchellh/go-ps"
	"gopkg.in/yaml.v2"
)

type K0sStatus struct {
	Version  string
	Pid      int
	PPid     int
	Role     string
	SysInit  string
	StubFile string
	Output   string
}

func GetPid() (status *K0sStatus, err error) {
	pid, ppid, err := getProcessID()
	if err == nil && pid != nil {
		status = &K0sStatus{
			Pid:  *pid,
			PPid: *ppid,
		}
		return status, nil
	}
	return &K0sStatus{}, nil
}

func getProcessID() (pid *int, ppid *int, err error) {
	processList, err := ps.Processes()
	if err != nil {
		return nil, nil, err
	}

	for _, p := range processList {
		if p.Executable() == "k0s" && hasChildren(p.Pid(), processList) {
			pid := p.Pid()
			ppid := p.PPid()
			return &pid, &ppid, nil
		}
	}
	return nil, nil, nil
}

func hasChildren(pid int, processes []ps.Process) bool {
	for _, p := range processes {
		if p.PPid() == pid {
			return true
		}
	}
	return false
}

func GetRoleByPID(pid int) (role string, err error) {
	if runtime.GOOS == "windows" {
		return "worker", nil
	}

	var raw []byte
	if raw, err = ioutil.ReadFile(fmt.Sprintf("/proc/%d/cmdline", pid)); err != nil {
		return "", err
	}
	cmdln := string(raw)
	if strings.Contains(cmdln, "enable-worker") {
		return "controller+worker", nil
	} else if strings.Contains(cmdln, "controller") {
		return "controller", nil
	} else if strings.Contains(cmdln, "worker") {
		return "worker", nil
	} else if strings.Contains(cmdln, "server") {
		return "controller", nil
	}
	return "", fmt.Errorf("k0s role is not found")
}

func (s K0sStatus) GetK0sVersion() (string, error) {
	cmd := fmt.Sprintf("/proc/%d/exe", s.Pid)
	stdout, err := exec.Command(cmd, "version").Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSuffix(string(stdout), "\n"), nil
}

func (s K0sStatus) String() {
	switch s.Output {
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
