// SPDX-FileCopyrightText: 2024 k0s authors
// SPDX-License-Identifier: Apache-2.0

package keepalived

import (
	"bytes"
	"context"
	"fmt"
	"html/template"
	"net"
	"os"
	"strings"
	"testing"

	"github.com/k0sproject/k0s/inttest/common"

	"github.com/stretchr/testify/suite"
)

type cplbIPVSSuite struct {
	common.BootlooseSuite
	isIPv6Only bool
}

const keepalivedTestMarker = "# k0s-inttest-cplb-ipvs"

const cplbCfgTemplate = `
spec:
  network:
{{ if .isIPv6Only }}
    podCIDR: fd00::/108
    serviceCIDR: fd01::/108
{{ end }}
    controlPlaneLoadBalancing:
      enabled: true
      type: Keepalived
      keepalived:
        configTemplateVRRP: /tmp/keepalived-vrrp.conf
        configTemplateVS: /tmp/keepalived-virtualservers.conf
        vrrpInstances:
        - virtualIPs: ["{{ .lbCIDR }}"]
          authPass: "123456"
          unicastSourceIP: {{ .unicastSourceIP }}
          unicastPeers:
{{- range .unicastPeers }}
          - {{ . }}
{{- end }}
        virtualServers:
        - ipAddress: {{ .lbAddr }}
    nodeLocalLoadBalancing:
      enabled: true
      type: EnvoyProxy
`

func (s *cplbIPVSSuite) getK0sCfg(nodeIdx int, vip string) string {
	k0sCfg := bytes.NewBuffer([]byte{})
	srcIP, peers := s.getUnicastAddresses(nodeIdx, vip)
	data := map[string]any{
		"isIPv6Only":      s.isIPv6Only,
		"lbAddr":          vip,
		"lbCIDR":          common.GetCPLBVIPCIDR(vip, s.isIPv6Only),
		"unicastSourceIP": srcIP,
		"unicastPeers":    peers,
	}
	s.Require().NoError(template.Must(template.New("k0s.yaml").
		Parse(cplbCfgTemplate)).
		Execute(k0sCfg, data), "can't execute k0s.yaml template")
	return k0sCfg.String()
}

// generateKeepalivedTemplates creates keepalived configuration files by executing
// k0s keepalived-config commands on the controller and appending an inttest marker.
func (s *cplbIPVSSuite) generateKeepalivedTemplates(ctx context.Context, idx int) {
	ssh, err := s.SSH(ctx, s.ControllerNode(idx))
	s.Require().NoError(err)
	defer ssh.Disconnect()

	vsOutput, err := ssh.ExecWithOutput(ctx, "/usr/local/bin/k0s keepalived-config virtualservers")
	s.Require().NoError(err)
	vsContent := vsOutput + "\n" + keepalivedTestMarker
	s.PutFile(s.ControllerNode(idx), "/tmp/keepalived-virtualservers.conf", vsContent)

	vrrpOutput, err := ssh.ExecWithOutput(ctx, "/usr/local/bin/k0s keepalived-config vrrp")
	s.Require().NoError(err)
	vrrpContent := vrrpOutput + "\n" + keepalivedTestMarker
	s.PutFile(s.ControllerNode(idx), "/tmp/keepalived-vrrp.conf", vrrpContent)
}

// verifyCustomTemplate verifies that the custom template marker is present in the
// generated keepalived configuration files on the controller node.
func (s *cplbIPVSSuite) verifyCustomTemplate(ctx context.Context, idx int) {
	ssh, err := s.SSH(ctx, s.ControllerNode(idx))
	s.Require().NoError(err)
	defer ssh.Disconnect()

	// Verify keepalived.conf contains the marker
	keepalivedConf, err := ssh.ExecWithOutput(ctx, "cat /run/k0s/keepalived.conf")
	s.Require().NoError(err)
	s.Require().Contains(keepalivedConf, keepalivedTestMarker, "keepalived.conf should contain the custom template marker")

	// Verify keepalived-virtualservers-generated.conf contains the marker
	vsConf, err := ssh.ExecWithOutput(ctx, "cat /run/k0s/keepalived-virtualservers-generated.conf")
	s.Require().NoError(err)
	s.Require().Contains(vsConf, keepalivedTestMarker, "keepalived-virtualservers-generated.conf should contain the custom template marker")
}

// SetupTest prepares the controller and filesystem, getting it into a consistent
// state which we can run tests against.
func (s *cplbIPVSSuite) TestK0sGetsUp() {
	if s.isIPv6Only {
		s.T().Log("Running on IPv6 mode")
	}

	vip := common.GetCPLBVIP(&s.BootlooseSuite, s.isIPv6Only)
	ctx := s.Context()
	var joinToken string

	for idx := range s.ControllerCount {
		s.T().Logf("getting config")
		k0sCfg := s.getK0sCfg(idx, vip)
		s.T().Logf("putting files")
		s.generateKeepalivedTemplates(ctx, idx)
		s.PutFile(s.ControllerNode(idx), "/tmp/k0s.yaml", k0sCfg)

		s.T().Logf("init controller")
		// Note that the token is intentionally empty for the first controller
		s.Require().NoError(s.InitController(idx,
			"--config=/tmp/k0s.yaml",
			"--disable-components=metrics-server",
			"--feature-gates=IPv6SingleStack=true",
			joinToken))
		s.Require().NoError(s.WaitJoinAPI(s.ControllerNode(idx)))

		s.T().Logf("waiting node")
		// With the primary controller running, create the join token for subsequent controllers.
		if idx == 0 {
			s.T().Logf("getting join token")
			token, err := s.GetJoinToken("controller")
			s.Require().NoError(err)
			joinToken = token
		}
	}

	s.T().Logf("getting members")
	// Final sanity -- ensure all nodes see each other according to etcd
	for idx := range s.ControllerCount {
		s.Require().Len(s.GetMembers(idx), s.ControllerCount)
	}

	// Create a worker join token
	workerJoinToken, err := s.GetJoinToken("worker")
	s.Require().NoError(err)

	// Start the workers using the join token
	s.Require().NoError(s.RunWorkersWithToken(workerJoinToken))

	client, err := s.KubeClient(s.ControllerNode(0))
	s.Require().NoError(err)

	s.Require().NoError(s.WaitForNodeReady(s.WorkerNode(0), client))

	// Verify that all servers have the dummy interface
	for idx := range s.ControllerCount {
		s.checkDummy(ctx, s.ControllerNode(idx), vip)
	}

	// Verify that only one controller has the VIP in eth0
	activeNode := -1
	count := 0
	for idx := range s.ControllerCount {
		if s.hasVIP(ctx, s.ControllerNode(idx), vip) {
			activeNode = idx
			count++
		}
	}
	s.Require().Equal(1, count, "Expected exactly one controller to have the VIP")

	// Verify that the real servers are present in the ipvsadm output in the active node and missing in the others
	for idx := range s.ControllerCount {
		s.validateRealServers(ctx, s.ControllerNode(idx), vip, idx == activeNode)
	}

	// Verify that the custom templates were used to generate the keepalived configuration
	for idx := range s.ControllerCount {
		s.verifyCustomTemplate(ctx, idx)
	}
}

// getUnicastAddresses returns the unicast addresses for the given index and
// slice with the IP addresses of the next two controllers.
func (s *cplbIPVSSuite) getUnicastAddresses(i int, cplbVIP string) (string, []string) {
	getAddr := func(i int) string {
		if s.isIPv6Only {
			return common.FirstPublicIPv6Address(&s.BootlooseSuite, s.ControllerNode(i%s.ControllerCount), cplbVIP)
		}
		return s.GetIPAddress(s.ControllerNode(i % s.ControllerCount))
	}

	return getAddr(i % s.ControllerCount), []string{
		getAddr((i + 1) % s.ControllerCount),
		getAddr((i + 2) % s.ControllerCount),
	}
}

// validateRealServers checks that the real servers are present in the
// ipvsadm output.
func (s *cplbIPVSSuite) validateRealServers(ctx context.Context, node string, vip string, isActive bool) {
	ssh, err := s.SSH(ctx, node)
	s.Require().NoError(err)
	defer ssh.Disconnect()

	servers := []string{}
	for i := range s.ControllerCount {
		if s.isIPv6Only {
			servers = append(servers, common.FirstPublicIPv6Address(&s.BootlooseSuite, s.ControllerNode(i), vip))
		} else {
			servers = append(servers, s.GetIPAddress(s.ControllerNode(i)))
		}
	}

	output, err := ssh.ExecWithOutput(ctx, "ipvsadm --save -n")
	s.Require().NoError(err)

	vipHostPort := net.JoinHostPort(vip, "6443")
	for _, server := range servers {
		serverHostPort := net.JoinHostPort(server, "6443")

		expected := fmt.Sprintf("-a -t %s -r %s", vipHostPort, serverHostPort)
		if isActive {
			s.Require().Containsf(output, expected, "Controller %s is missing a server in ipvsadm", node)
		} else {
			s.Require().NotContainsf(output, expected, "Controller %s has a server in ipvsadm", node)
		}
	}
}

// checkDummy checks that the dummy interface is present on the given node and
// that it has only the virtual IP address.
func (s *cplbIPVSSuite) checkDummy(ctx context.Context, node string, vip string) {
	ssh, err := s.SSH(ctx, node)
	s.Require().NoError(err)
	defer ssh.Disconnect()

	output, err := ssh.ExecWithOutput(ctx, "ip --oneline addr show dummyvip0")
	s.Require().NoError(err)

	s.Require().Equal(0, strings.Count(output, "\n"), "Expected only one line of output")

	expected := fmt.Sprintf("inet %s/32", vip)
	if s.isIPv6Only {
		expected = fmt.Sprintf("inet6 %s/128", vip)
	}
	s.Require().Contains(output, expected)
}

// hasVIP checks that the dummy interface is present on the given node and
// that it has only the virtual IP address.
func (s *cplbIPVSSuite) hasVIP(ctx context.Context, node string, vip string) bool {
	ssh, err := s.SSH(ctx, node)
	s.Require().NoError(err)
	defer ssh.Disconnect()

	output, err := ssh.ExecWithOutput(ctx, "ip --oneline addr show eth0")
	s.Require().NoError(err)

	if s.isIPv6Only {
		return strings.Contains(output, fmt.Sprintf("inet6 %s/64", vip))
	}
	return strings.Contains(output, fmt.Sprintf("inet %s/16", vip))
}

// TestKeepAlivedSuite runs the keepalived test suite. It verifies that the
// virtual IP is working by joining a node to the cluster using the VIP.
func TestCPLBIPVSSuite(t *testing.T) {
	s := &cplbIPVSSuite{
		BootlooseSuite: common.BootlooseSuite{
			ControllerCount: 3,
			WorkerCount:     1,
		},
	}

	if strings.Contains(os.Getenv("K0S_INTTEST_TARGET"), "ipv6") {
		t.Log("Configuring IPv6 only networking")
		s.isIPv6Only = true
		s.Networks = []string{"bridge-ipv6"}
		s.AirgapImageBundleMountPoints = []string{"/var/lib/k0s/images/bundle-ipv6.tar"}
	}
	suite.Run(t, s)
}
