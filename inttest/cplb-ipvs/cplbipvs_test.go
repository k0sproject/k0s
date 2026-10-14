// SPDX-FileCopyrightText: 2024 k0s authors
// SPDX-License-Identifier: Apache-2.0

package keepalived

import (
	"context"
	"fmt"
	"net"
	"os"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/k0sproject/k0s/inttest/common"
	"github.com/k0sproject/k0s/pkg/apis/k0s/v1beta1"

	"github.com/stretchr/testify/suite"
)

type cplbIPVSSuite struct {
	common.BootlooseSuite
	isIPv6Only bool
	// Whether the konnectivity agent and k0s API ports are load balanced as well.
	loadBalanceAllPorts bool
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
{{ .keepalived }}
    nodeLocalLoadBalancing:
      enabled: true
      type: EnvoyProxy
`

// getK0sCfg renders the k0s config with the given keepalived settings.
func (s *cplbIPVSSuite) getK0sCfg(nodeIdx int, vip string, spec *v1beta1.KeepalivedSpec) string {
	srcIP, peers := s.getUnicastAddresses(nodeIdx, vip)
	data := map[string]any{
		"isIPv6Only":      s.isIPv6Only,
		"keepalived":      s.KeepalivedSettings(spec),
		"lbAddr":          vip,
		"lbCIDR":          common.GetCPLBVIPCIDR(vip, s.isIPv6Only),
		"unicastSourceIP": srcIP,
		"unicastPeers":    peers,
	}
	return s.RenderTemplate("k0s.yaml", cplbCfgTemplate, data)
}

// generateKeepalivedTemplates creates keepalived configuration files by executing
// k0s keepalived-config commands on the controller and appending an inttest marker.
func (s *cplbIPVSSuite) generateKeepalivedTemplates(idx int) {
	vsOutput, err := s.RunCommandController(idx, "/usr/local/bin/k0s keepalived-config virtualservers")
	s.Require().NoError(err)
	vsContent := vsOutput + "\n" + keepalivedTestMarker
	s.PutFile(s.ControllerNode(idx), "/tmp/keepalived-virtualservers.conf", vsContent)

	vrrpOutput, err := s.RunCommandController(idx, "/usr/local/bin/k0s keepalived-config vrrp")
	s.Require().NoError(err)
	vrrpContent := vrrpOutput + "\n" + keepalivedTestMarker
	s.PutFile(s.ControllerNode(idx), "/tmp/keepalived-vrrp.conf", vrrpContent)
}

// verifyCustomTemplate verifies that the custom template marker is present in the
// generated keepalived configuration files on the controller node.
func (s *cplbIPVSSuite) verifyCustomTemplate(idx int) {
	// Verify keepalived.conf contains the marker
	keepalivedConf, err := s.RunCommandController(idx, "cat /run/k0s/keepalived.conf")
	s.Require().NoError(err)
	s.Require().Contains(keepalivedConf, keepalivedTestMarker, "keepalived.conf should contain the custom template marker")

	// Verify keepalived-virtualservers-generated.conf contains the marker
	vsConf, err := s.RunCommandController(idx, "cat /run/k0s/keepalived-virtualservers-generated.conf")
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
	// Keepalived settings for the API server port only, and for all ports.
	specs := map[bool]*v1beta1.KeepalivedSpec{false: {}, true: common.AllPortsKeepalivedSpec()}
	var joinToken string

	for idx := range s.ControllerCount {
		s.T().Logf("getting config")
		k0sCfg := s.getK0sCfg(idx, vip, specs[s.loadBalanceAllPorts])
		s.T().Logf("putting files")
		s.generateKeepalivedTemplates(idx)
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
	s.Require().Len(common.VIPHolders(ctx, &s.BootlooseSuite, vip, s.isIPv6Only), 1, "Expected exactly one controller to have the VIP")

	// Verify that the real servers are present in the ipvsadm output in the active node and missing in the others
	s.checkRealServers(ctx, vip, specs[s.loadBalanceAllPorts])

	// Verify that the custom templates were used to generate the keepalived configuration
	for idx := range s.ControllerCount {
		s.verifyCustomTemplate(idx)
	}

	// Restarts move the virtual IP, so each phase also checks that only the new holder load balances.
	for _, phase := range []struct {
		spec *v1beta1.KeepalivedSpec
		name string
	}{
		{specs[!s.loadBalanceAllPorts], "the other way around"},
		{&v1beta1.KeepalivedSpec{DisableLoadBalancer: true, LoadBalancedPorts: common.AllPortsKeepalivedSpec().LoadBalancedPorts}, "disabled load balancer"},
	} {
		s.Run(phase.name, func() {
			s.RestartControllersWithConfig("/tmp/k0s.yaml", func(idx int) string { return s.getK0sCfg(idx, vip, phase.spec) })
			s.checkRealServers(ctx, vip, phase.spec)
		})
	}
}

// getUnicastAddresses returns the unicast addresses for the given index and
// slice with the IP addresses of the next two controllers.
func (s *cplbIPVSSuite) getUnicastAddresses(i int, cplbVIP string) (string, []string) {
	getAddr := func(i int) string {
		return common.ControllerAddress(&s.BootlooseSuite, i%s.ControllerCount, cplbVIP, s.isIPv6Only)
	}

	return getAddr(i % s.ControllerCount), []string{
		getAddr((i + 1) % s.ControllerCount),
		getAddr((i + 2) % s.ControllerCount),
	}
}

// checkRealServers waits until one controller has the virtual IP and load balances the ports of the spec to all
// controllers. The other controllers must not load balance anything.
func (s *cplbIPVSSuite) checkRealServers(ctx context.Context, vip string, spec *v1beta1.KeepalivedSpec) {
	ports := s.BalancedPorts(spec)
	check := func() error {
		holders := common.VIPHolders(ctx, &s.BootlooseSuite, vip, s.isIPv6Only)
		if len(holders) != 1 {
			return fmt.Errorf("controllers %v hold the virtual IP instead of exactly one", holders)
		}
		for idx := range s.ControllerCount {
			node := s.ControllerNode(idx)
			if err := s.validateRealServers(ctx, node, vip, node == holders[0], ports); err != nil {
				return err
			}
		}
		return nil
	}

	deadline := time.Now().Add(3 * time.Minute)
	for err := check(); err != nil; err = check() {
		s.Require().False(time.Now().After(deadline), "%v", err)
		select {
		case <-ctx.Done():
			s.Require().NoError(ctx.Err())
		case <-time.After(time.Second):
		}
	}
}

// validateRealServers checks that the real servers are present in the
// ipvsadm output for the given ports, and that no other port is load balanced.
func (s *cplbIPVSSuite) validateRealServers(ctx context.Context, node string, vip string, isActive bool, ports []v1beta1.BalancedPort) error {
	ssh, err := s.SSH(ctx, node)
	s.Require().NoError(err)
	defer ssh.Disconnect()

	servers := []string{}
	for i := range s.ControllerCount {
		servers = append(servers, common.ControllerAddress(&s.BootlooseSuite, i, vip, s.isIPv6Only))
	}

	output, err := ssh.ExecWithOutput(ctx, "ipvsadm --save -n")
	s.Require().NoError(err)

	for _, port := range []int{s.KubeAPIExternalPort, s.KonnectivityAgentPort, s.K0sAPIExternalPort} {
		vipHostPort := net.JoinHostPort(vip, strconv.Itoa(port))
		if !slices.ContainsFunc(ports, func(p v1beta1.BalancedPort) bool { return p.Port == port }) {
			if strings.Contains(output, "-A -t "+vipHostPort+" ") {
				return fmt.Errorf("controller %s load balances port %d", node, port)
			}
			continue
		}
		for _, server := range servers {
			serverHostPort := net.JoinHostPort(server, strconv.Itoa(port))

			expected := fmt.Sprintf("-a -t %s -r %s", vipHostPort, serverHostPort)
			if isActive && !strings.Contains(output, expected) {
				return fmt.Errorf("controller %s is missing a server in ipvsadm", node)
			} else if !isActive && strings.Contains(output, expected) {
				return fmt.Errorf("controller %s has a server in ipvsadm", node)
			}
		}
	}
	return nil
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
		// The userspace suite covers IPv6 with only the API server port.
		s.loadBalanceAllPorts = true
	}
	suite.Run(t, s)
}
