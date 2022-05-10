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
	"testing"

	"github.com/stretchr/testify/suite"
	"sigs.k8s.io/yaml"

	"github.com/k0sproject/k0s/inttest/common"
	"github.com/k0sproject/k0sctl/integration/github"
)

type K0sctlSuite struct {
	common.FootlooseSuite
}

func (s *K0sctlSuite) haveLatest(latest github.Release) bool {
	out, err := exec.Command("./k0sctl", "version").Output()
	if err != nil {
		return false
	}
	return strings.Contains(string(out), fmt.Sprintf("%s\n", latest.TagName))
}

func (s *K0sctlSuite) k0sctlFilename() string {
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

func (s *K0sctlSuite) k0sctlDownloadAsset(latest github.Release) (github.Asset, error) {
	fn := s.k0sctlFilename()
	for _, a := range latest.Assets {
		if a.Name == fn {
			return a, nil
		}
	}
	return github.Asset{}, fmt.Errorf("failed to find a k0sctl release binary asset %s", fn)
}

func (s *K0sctlSuite) DownloadK0sctl() error {
	latest, err := github.LatestRelease("k0sproject/k0sctl", false)
	if err != nil {
		return fmt.Errorf("failed to get latest k0sctl version from github: %w", err)
	}

	if s.haveLatest(latest) {
		s.T().Logf("Already have k0sctl %s", latest.TagName)
		return nil
	}

	s.T().Logf("Downloading k0sctl %s", latest.TagName)

	asset, err := s.k0sctlDownloadAsset(latest)
	if err != nil {
		return err
	}
	s.T().Logf("Found matching asset: %s - Downloading", asset.Name)

	req, err := http.NewRequest("GET", asset.URL, nil)
	if err != nil {
		return err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}

	defer resp.Body.Close()

	f, err := os.OpenFile("k0sctl", os.O_CREATE|os.O_WRONLY, 0755)
	if err != nil {
		return err
	}
	defer f.Close()

	_, err = io.Copy(f, resp.Body)
	s.T().Logf("Download of %s complete", f.Name())
	return err
}

func (s *K0sctlSuite) k0sctlInitConfig() (map[string]interface{}, error) {
	nodes := make([]string, s.ControllerCount+s.WorkerCount)
	addresses := make([]string, s.ControllerCount+s.WorkerCount)
	for i := 0; i < s.ControllerCount; i++ {
		nodes[i] = s.ControllerNode(i)
	}
	for i := 0; i < s.WorkerCount; i++ {
		nodes[i+s.ControllerCount] = s.WorkerNode(i)
	}

	machines, err := s.InspectMachines(nodes)
	if err != nil {
		return nil, err
	}

	for _, m := range machines {
		port, err := m.HostPort(22)
		s.NoError(err)
		addresses = append(addresses, fmt.Sprintf("127.0.0.1:%d", port))
	}

	ssh, err := s.SSH(nodes[0])
	if err != nil {
		s.FailNow("ssh connection failed", "%s", err)
	}
	args := []string{"init", "--controller-count", fmt.Sprintf("%d", s.ControllerCount), "--key-path", ssh.KeyPath, "--user", ssh.User}
	args = append(args, addresses...)
	out, err := exec.Command("./k0sctl", args...).Output()
	s.NoError(err)

	cfg := map[string]interface{}{}
	err = yaml.Unmarshal(out, &cfg)
	return cfg, err
}

func (s *K0sctlSuite) k0sctlApply(cfg map[string]interface{}) error {
	plain, err := yaml.Marshal(cfg)
	if err != nil {
		return err
	}
	cmd := exec.Command("./k0sctl", "apply", "--config", "-")
	cmd.Stdin = bytes.NewReader(plain)

	stdout, _ := cmd.StdoutPipe()
	if err := cmd.Start(); err != nil {
		return err
	}

	scanner := bufio.NewScanner(stdout)
	for scanner.Scan() {
		m := scanner.Text()
		s.T().Log(m)
	}
	return cmd.Wait()
}

func (s *K0sctlSuite) TestK0sGetsUp() {
	s.NoError(s.DownloadK0sctl())
	cfg, err := s.k0sctlInitConfig()

	spec, ok := cfg["spec"].(map[string]interface{})
	if !ok {
		s.FailNow("could not find spec in generated k0sctl.yaml")
	}
	hosts, ok := spec["hosts"].([]interface{})
	if !ok {
		s.FailNow("could not find spec.hosts in generated k0sctl.yaml")
	}

	for _, host := range hosts {
		h, ok := host.(map[string]interface{})
		if !ok {
			s.FailNow("host not what was expectd")
		}
		h["uploadBinary"] = true
		h["k0sBinaryPath"] = os.Getenv("K0S_PATH")
	}

	s.NoError(err)
	s.NoError(s.k0sctlApply(cfg))
}

func TestK0sctlSuite(t *testing.T) {
	s := K0sctlSuite{
		common.FootlooseSuite{
			ControllerCount: 1,
			WorkerCount:     1,
		},
	}
	suite.Run(t, &s)
}
