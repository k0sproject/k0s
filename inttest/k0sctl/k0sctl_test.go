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

package k0sctl

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/suite"
	"sigs.k8s.io/yaml"

	"github.com/k0sproject/k0s/inttest/common"
)

const k0sctlVersion = "v0.13.0"

type K0sctlSuite struct {
	common.FootlooseSuite
	k0sctlEnv []string
}

func (s *K0sctlSuite) haveLatest() bool {
	cmd := exec.Command("./k0sctl", "version")
	cmd.Env = s.k0sctlEnv
	out, err := cmd.Output()
	if err != nil {
		return false
	}
	return strings.Contains(string(out), fmt.Sprintf("%s\n", k0sctlVersion))
}

func k0sctlFilename() string {
	var ext string
	os := runtime.GOOS
	if os == "windows" {
		os = "win"
		ext = ".exe"
	}

	var arch string
	switch runtime.GOARCH {
	case "amd64":
		arch = "x64"
	case "arm64", "arm64be":
		arch = "arm64"
	case "amd64p32", "arm", "armbe":
		arch = "arm"
	default:
		arch = runtime.GOARCH
	}
	return fmt.Sprintf("k0sctl-%s-%s%s", os, arch, ext)
}

func (s *K0sctlSuite) downloadK0sctl() {
	if s.haveLatest() {
		s.T().Logf("Already have k0sctl %s", k0sctlVersion)
		return
	}

	s.T().Logf("Downloading k0sctl %s", k0sctlVersion)

	req, err := http.NewRequest("GET", fmt.Sprintf("https://github.com/k0sproject/k0sctl/releases/download/%s/%s", k0sctlVersion, k0sctlFilename()), nil)
	s.Require().NoError(err)
	resp, err := http.DefaultClient.Do(req)
	s.Require().NoError(err)

	defer resp.Body.Close()

	f, err := os.OpenFile("k0sctl", os.O_CREATE|os.O_WRONLY, 0755)
	s.Require().NoError(err)
	defer f.Close()

	_, err = io.Copy(f, resp.Body)
	s.Require().NoError(err)
	s.T().Logf("Download of %s complete", f.Name())
}

func (s *K0sctlSuite) k0sctlInitConfig() map[string]interface{} {
	nodes := make([]string, s.ControllerCount+s.WorkerCount)
	addresses := make([]string, s.ControllerCount+s.WorkerCount)
	for i := 0; i < s.ControllerCount; i++ {
		nodes[i] = s.ControllerNode(i)
	}
	for i := 0; i < s.WorkerCount; i++ {
		nodes[i+s.ControllerCount] = s.WorkerNode(i)
	}

	machines, err := s.InspectMachines(nodes)
	s.Require().NoError(err)

	for _, m := range machines {
		port, err := m.HostPort(22)
		s.Require().NoError(err)
		addresses = append(addresses, fmt.Sprintf("127.0.0.1:%d", port))
	}

	ssh, err := s.SSH(s.Context(), nodes[0])
	if err != nil {
		s.FailNow("ssh connection failed", "%s", err)
	}
	args := []string{"init", "--controller-count", fmt.Sprintf("%d", s.ControllerCount), "--key-path", ssh.KeyPath, "--user", ssh.User}
	args = append(args, addresses...)
	cmd := exec.Command("./k0sctl", args...)
	cmd.Env = s.k0sctlEnv
	out, err := cmd.Output()
	s.Require().NoError(err)

	cfg := map[string]interface{}{}
	err = yaml.Unmarshal(out, &cfg)

	s.Require().NoError(err)
	return cfg
}

func (s *K0sctlSuite) k0sctlApply(cfg map[string]interface{}) {
	plain, err := yaml.Marshal(cfg)
	s.Require().NoError(err)

	s.T().Logf("Applying k0sctl config:\n%s", plain)
	cmd := exec.Command("./k0sctl", "apply", "--config", "-")
	cmd.Env = s.k0sctlEnv
	cmd.Stdin = bytes.NewReader(plain)

	stdout, err := cmd.StdoutPipe()
	s.Require().NoError(err)
	stderr, err := cmd.StderrPipe()
	s.Require().NoError(err)

	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		defer wg.Done()
		scanner := bufio.NewScanner(stdout)
		for scanner.Scan() {
			s.T().Log(scanner.Text())
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		scanner := bufio.NewScanner(stderr)
		for scanner.Scan() {
			s.T().Log("STDERR", scanner.Text())
		}
	}()

	s.Require().NoError(cmd.Start())

	err = cmd.Wait()
	wg.Wait()
	s.Require().NoError(err)
}

func (s *K0sctlSuite) TestK0sGetsUp() {
	k0sBinaryPath := os.Getenv("K0S_PATH")
	k0sVersion, err := exec.Command(k0sBinaryPath, "version").Output()
	s.Require().NoError(err, "failed to get k0s version")

	s.downloadK0sctl()
	cfg := s.k0sctlInitConfig()

	spec, ok := cfg["spec"].(map[string]interface{})
	s.Require().True(ok, "could not find spec in generated k0sctl.yaml")
	hosts, ok := spec["hosts"].([]interface{})
	s.Require().True(ok, "could not find spec.hosts in generated k0sctl.yaml")

	for _, host := range hosts {
		h, ok := host.(map[string]interface{})
		s.Require().True(ok, "host not what was expected")
		h["uploadBinary"] = true
		h["k0sBinaryPath"] = k0sBinaryPath
	}

	k0s, ok := spec["k0s"].(map[string]interface{})
	s.Require().True(ok, "could not find spec.k0s in generated k0sctl.yaml")

	k0s["version"] = strings.TrimSpace(string(k0sVersion))

	s.k0sctlApply(cfg)
}

func TestK0sctlSuite(t *testing.T) {
	s := K0sctlSuite{
		common.FootlooseSuite{
			ControllerCount: 1,
			WorkerCount:     1,
		},
		[]string{
			fmt.Sprintf("USER=%s", t.Name()),
			fmt.Sprintf("HOME=%s", t.TempDir()),
			fmt.Sprintf("TMPDIR=%s", t.TempDir()),
		},
	}
	suite.Run(t, &s)
}
