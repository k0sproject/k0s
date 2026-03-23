//go:build linux

// SPDX-FileCopyrightText: 2026 k0s authors
// SPDX-License-Identifier: Apache-2.0

package privilegedport

import (
	"bytes"
	"context"
	"fmt"
	"html/template"
	"strconv"
	"strings"
	"testing"

	"github.com/k0sproject/k0s/inttest/common"
	"github.com/stretchr/testify/suite"
	"golang.org/x/sys/unix"
)

type PrivilegedPortSuite struct {
	common.BootlooseSuite
}

const configWithPrivilegedPort = `
apiVersion: k0s.k0sproject.io/v1beta1
kind: ClusterConfig
metadata:
  name: k0s
spec:
  api:
    port: {{ .Port }}
`

const privilegedPort = 443

func TestPrivilegedPortSuite(t *testing.T) {
	s := PrivilegedPortSuite{
		common.BootlooseSuite{
			ControllerCount:     1,
			WorkerCount:         0,
			KubeAPIExternalPort: privilegedPort,
		},
	}
	suite.Run(t, &s)
}

func (s *PrivilegedPortSuite) getControllerConfig() string {
	data := struct {
		Port int
	}{
		Port: privilegedPort,
	}
	content := bytes.NewBuffer([]byte{})
	s.Require().NoError(template.Must(template.New("k0s.yaml").Parse(configWithPrivilegedPort)).Execute(content, data), "can't execute k0s.yaml template")
	return content.String()
}

func (s *PrivilegedPortSuite) TestCapNetBindServiceIsSet() {
	ctx := s.Context()
	
	// Setup k0s with privileged port configuration
	config := s.getControllerConfig()
	s.PutFile(s.ControllerNode(0), "/tmp/k0s.yaml", config)
	
	s.Require().NoError(s.InitController(0, "--config=/tmp/k0s.yaml"))
	
	kc, err := s.KubeClient(s.ControllerNode(0))
	s.Require().NoError(err)
	
	s.AssertSomeKubeSystemPods(kc)
	
	// Now verify that kube-apiserver has CAP_NET_BIND_SERVICE
	s.Run("kube-apiserver has CAP_NET_BIND_SERVICE", func() {
		s.Require().NoError(s.verifyCapability(ctx, s.ControllerNode(0)))
	})
}

// verifyCapability checks if the kube-apiserver process has CAP_NET_BIND_SERVICE capability
func (s *PrivilegedPortSuite) verifyCapability(ctx context.Context, node string) error {
	ssh, err := s.SSH(ctx, node)
	if err != nil {
		return fmt.Errorf("failed to SSH to node: %w", err)
	}
	defer ssh.Disconnect()
	
	// Find the kube-apiserver PID
	pid, err := ssh.ExecWithOutput(ctx, "pidof kube-apiserver")
	if err != nil {
		return fmt.Errorf("failed to find kube-apiserver process: %w", err)
	}
	pid = strings.TrimSpace(pid)
	if pid == "" {
		return fmt.Errorf("kube-apiserver process not found")
	}
	
	s.T().Logf("Found kube-apiserver with PID: %s", pid)
	
	// Read the capability information from /proc/<pid>/status
	// We need to check CapEff (effective capabilities)
	capOutput, err := ssh.ExecWithOutput(ctx, fmt.Sprintf("grep CapEff /proc/%s/status", pid))
	if err != nil {
		return fmt.Errorf("failed to read capabilities: %w", err)
	}
	
	s.T().Logf("Capability output: %s", capOutput)
	
	// Parse the capability hex value
	// Format is "CapEff:\t0000000000000400" (or similar)
	parts := strings.Fields(capOutput)
	if len(parts) < 2 {
		return fmt.Errorf("unexpected capability format: %s", capOutput)
	}
	
	capHex := parts[1]
	capValue, err := strconv.ParseUint(capHex, 16, 64)
	if err != nil {
		return fmt.Errorf("failed to parse capability value %s: %w", capHex, err)
	}
	
	// Check if CAP_NET_BIND_SERVICE (bit 10) is set
	// unix.CAP_NET_BIND_SERVICE is defined in golang.org/x/sys/unix
	capNetBindService := uint64(unix.CAP_NET_BIND_SERVICE)
	if capValue&(1<<capNetBindService) == 0 {
		return fmt.Errorf("CAP_NET_BIND_SERVICE (bit %d) is not set in capabilities: 0x%x", capNetBindService, capValue)
	}
	
	s.T().Logf("CAP_NET_BIND_SERVICE is correctly set (capability value: 0x%x, bit %d is set)", capValue, capNetBindService)
	return nil
}
