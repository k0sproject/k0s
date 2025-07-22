// SPDX-FileCopyrightText: 2025 k0s authors
// SPDX-License-Identifier: Apache-2.0

package ipv6

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/k0sproject/k0s/inttest/common"
	"github.com/k0sproject/k0s/pkg/airgap"
	"github.com/k0sproject/k0s/pkg/apis/k0s/v1beta1"

	"github.com/stretchr/testify/suite"
)

const ipv6ResolvConf = `
nameserver 2606:4700:4700::1111
nameserver 2001:4860:4860::8888
`

const k0sConfigWithKuberouter = `
spec:
  images:
    default_pull_policy: Never
  api:
    address: %s
  network:
    provider: kuberouter
    podCIDR: fd00::/108
    serviceCIDR: fd01::/108
`

const k0sConfigWithCalico = `
spec:
  images:
    default_pull_policy: Never
  api:
    address: %s
  network:
    provider: calico
    podCIDR: fd00::/108
    serviceCIDR: fd01::/108
`

type IPv6Suite struct {
	common.BootlooseSuite
}

func (s *IPv6Suite) TestK0sGetsUp() {
	s.validateDockerBridge()
	ctx := s.Context()

	var k0sConfig, cniDS string

	if strings.Contains(os.Getenv("K0S_INTTEST_TARGET"), "kuberouter") {
		s.T().Log("Using kube-router network")
		k0sConfig = fmt.Sprintf(k0sConfigWithKuberouter, common.FirstPublicIPv6Address(&s.BootlooseSuite, s.ControllerNode(0), ""))
		cniDS = "kube-router"
	} else {
		s.T().Log("Using calico network")
		k0sConfig = fmt.Sprintf(k0sConfigWithCalico, common.FirstPublicIPv6Address(&s.BootlooseSuite, s.ControllerNode(0), ""))
		cniDS = "calico-node"
	}

	s.PutFile(s.ControllerNode(0), "/tmp/k0s.yaml", k0sConfig)

	// If there isn't a valid IPv6 DNS docker will write 127.0.0.11 which creates a routing loop.
	// Overwrite it with cloudflare's and google's IPv6 DNS. Technically we only need this on the workers.
	s.PutFile(s.WorkerNode(0), "/etc/resolv.conf", ipv6ResolvConf)

	s.NoError(s.InitController(0, "--config=/tmp/k0s.yaml"))
	s.NoError(s.RunWorkers(`--labels="k0sproject.io/foo=bar"`, `--kubelet-extra-args="--address=:: --event-burst=10 --image-gc-high-threshold=100"`))

	kc, err := s.KubeClient(s.ControllerNode(0))
	s.Require().NoError(err)
	s.T().Log("kube client acquired")

	err = s.WaitForNodeReady(s.WorkerNode(0), kc)
	s.NoError(err)

	if labels, err := s.GetNodeLabels(s.WorkerNode(0), kc); s.NoError(err) {
		s.Equal("bar", labels["k0sproject.io/foo"])
	}

	s.T().Logf("Waiting for %s to become ready", cniDS)
	s.Require().NoErrorf(common.WaitForDaemonSet(ctx, kc, cniDS, "kube-system"), "Waiting for %s to become ready", cniDS)
	s.T().Log("Waiting for coredns to become ready")
	s.Require().NoError(common.WaitForCoreDNSReady(ctx, kc), "While waiting for CoreDNS to become ready")
	s.T().Log("Waiting for logs to work")
	s.Require().NoError(common.WaitForPodLogs(ctx, kc, "kube-system"), "While waiting for some pod logs")

	// Check that all the images have io.cri-containerd.pinned=pinned label
	ssh, err := s.SSH(ctx, s.WorkerNode(0))
	s.Require().NoError(err)
	defer ssh.Disconnect()
	for _, i := range airgap.GetImageURIs(v1beta1.DefaultClusterSpec(), true) {
		output, err := ssh.ExecWithOutput(ctx, fmt.Sprintf(`k0s ctr i ls "name==%s"`, i))
		s.Require().NoError(err)
		s.Require().Containsf(output, "io.cri-containerd.pinned=pinned", "expected %s image to have io.cri-containerd.pinned=pinned label", i)
	}
}

func (s *IPv6Suite) validateDockerBridge() {
	cmd := exec.Command("docker", "network", "inspect", "bridge-ipv6",
		"--format", "'{{ .EnableIPv4 }},{{ .EnableIPv6 }}'")
	output, err := cmd.CombinedOutput()
	s.Require().NoError(err)
	s.Require().Containsf(string(output), "false,true", "expected bridge-ipv6 network to have IPv6 enabled")
}

func TestAirgapIPv6Suite(t *testing.T) {
	s := IPv6Suite{
		common.BootlooseSuite{
			ControllerCount: 1,
			WorkerCount:     1,
			// IPv6 only network is required.
			Networks: []string{"bridge-ipv6"},
			// technically this test doesn't need to be airgapped, but github workers don't have IPv6...
			AirgapImageBundleMountPoints: []string{"/var/lib/k0s/images/bundle.tar"},
		},
	}
	suite.Run(t, &s)
}
