// SPDX-FileCopyrightText: 2024 k0s authors
// SPDX-License-Identifier: Apache-2.0

package keepalived

import (
	"context"
	"fmt"
	"maps"
	"net"
	"net/url"
	"os"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/k0sproject/k0s/pkg/apis/k0s/v1beta1"
	"github.com/k0sproject/k0s/pkg/token"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	"github.com/k0sproject/k0s/inttest/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

type CPLBUserSpaceSuite struct {
	common.BootlooseSuite
	isIPv6Only         bool
	useExternalAddress bool
	// Whether the konnectivity agent and k0s API ports are load balanced as well.
	loadBalanceAllPorts bool
}

const nllbControllerConfig = `
spec:
  network:
    controlPlaneLoadBalancing:
      enabled: true
      type: Keepalived
      keepalived:
        vrrpInstances:
        - virtualIPs: ["{{ .VIP }}"]
          authPass: "123456"
{{ .Keepalived }}
    nodeLocalLoadBalancing:
      enabled: true
      type: EnvoyProxy
`

const extAddrControllerConfig = `
spec:
  images:
    # GitHub Actions don't support IPv6, for this reason in the test IPv6-only
    # we air gap the cluster but in IPv4 we don't. Setting the default pull
    # policy to "IfNotPresent" works on both environments.
    default_pull_policy: IfNotPresent
  api:
    externalAddress: {{ .ExtAddr }}
  network:
    provider: calico
    controlPlaneLoadBalancing:
      enabled: true
      type: Keepalived
      keepalived:
        vrrpInstances:
        - virtualIPs: ["{{ .VIP }}"]
          authPass: "123456"
          interface: "{{ .Interface }}"
{{ .Keepalived }}
{{ if .IsIPv6Only }}
    podCIDR: fd00::/108
    serviceCIDR: fd01::/108
{{ end }}
`

// getK0sCfg renders the k0s config with the given keepalived settings.
func (s *CPLBUserSpaceSuite) getK0sCfg(lb string, nic string, spec *v1beta1.KeepalivedSpec) string {
	cfg := nllbControllerConfig
	if s.useExternalAddress {
		cfg = extAddrControllerConfig
	}

	data := map[string]any{
		"ExtAddr":    lb,
		"VIP":        common.GetCPLBVIPCIDR(lb, s.isIPv6Only),
		"IsIPv6Only": s.isIPv6Only,
		"Interface":  nic,
		"Keepalived": s.KeepalivedSettings(spec),
	}
	return s.RenderTemplate("k0s.yaml", cfg, data)
}

// SetupTest prepares the controller and filesystem, getting it into a consistent
// state which we can run tests against.
func (s *CPLBUserSpaceSuite) TestK0sGetsUp() {
	lb := common.GetCPLBVIP(&s.BootlooseSuite, s.isIPv6Only)
	// Keepalived settings for the API server port only, and for all ports.
	specs := map[bool]*v1beta1.KeepalivedSpec{false: {}, true: common.AllPortsKeepalivedSpec()}

	ctx := s.Context()
	var joinToken string
	for idx := range s.ControllerCount {
		k0sCfg := s.getK0sCfg(lb, s.nic(idx), specs[s.loadBalanceAllPorts])
		s.PutFile(s.ControllerNode(idx), "/tmp/k0s.yaml", k0sCfg)
		if s.isIPv6Only {
			// Note that the token is intentionally empty for the first controller
			// Disable coreDNS to prevent crashloop backoff in github actions. This happens because
			// there is no IPv6 connectivity to the outside world in the CI environment and sets the DNS to ::1.
			s.Require().NoError(s.InitController(idx,
				"--config=/tmp/k0s.yaml",
				"--disable-components=endpoint-reconciler,coredns",
				"--feature-gates=IPv6SingleStack=true",
				"--enable-worker",
				joinToken))
		} else {
			// Note that the token is intentionally empty for the first controller
			s.Require().NoError(s.InitController(idx, "--config=/tmp/k0s.yaml", "--enable-worker", joinToken))
		}

		s.Require().NoError(s.WaitJoinAPI(s.ControllerNode(idx)))

		// With the primary controller running, create the join token for subsequent controllers.
		if idx == 0 {
			token, err := s.GetJoinToken("controller")
			s.Require().NoError(err)
			joinToken = token
		}
	}

	// Final sanity -- ensure all nodes see each other according to etcd
	for idx := range s.ControllerCount {
		s.Require().Len(s.GetMembers(idx), s.ControllerCount)
	}

	// Create a worker join token
	workerJoinToken, err := s.GetJoinToken(token.RoleWorker)
	s.Require().NoError(err)

	// Start the workers using the join token
	s.Require().NoError(s.RunWorkersWithToken(workerJoinToken))

	restConfig, err := s.GetKubeConfig(s.ControllerNode(0))
	s.Require().NoError(err)
	client, err := kubernetes.NewForConfig(restConfig)
	s.Require().NoError(err)

	for idx := range s.ControllerCount {
		s.Require().NoError(s.WaitForNodeReady(s.ControllerNode(idx), client))
	}
	s.Require().NoError(s.WaitForNodeReady(s.WorkerNode(0), client))

	// Verify that none of the servers has the dummy interface
	for idx := range s.ControllerCount {
		s.checkDummy(ctx, s.ControllerNode(idx))
	}

	// Verify that only one controller has the VIP in eth0
	s.Require().Len(common.VIPHolders(ctx, &s.BootlooseSuite, lb, s.isIPv6Only), 1, "Expected exactly one controller to have the VIP")

	// Verify that controller+worker nodes are working normally.
	s.T().Log("waiting to see CNI pods ready")
	if s.useExternalAddress {
		s.Require().NoError(common.WaitForDaemonSet(ctx, client, "calico-node", metav1.NamespaceSystem), "calico-node did not start")
	} else {
		s.Require().NoError(common.WaitForKubeRouterReady(ctx, client), "kube router did not start")
	}

	s.T().Log("waiting to see konnectivity-agent pods ready")
	s.Require().NoError(common.WaitForDaemonSet(ctx, client, "konnectivity-agent", metav1.NamespaceSystem), "konnectivity-agent did not start")
	{
		s.T().Log("waiting for konnectivity")
		controllerCount := uint(s.ControllerCount)
		// Agents connect through the external address. Unless its konnectivity port is load balanced, they reach one server.
		if s.useExternalAddress && !s.loadBalanceAllPorts {
			controllerCount = 1
		}
		s.Require().NoError(common.VerifyKonnectivityMesh(ctx, restConfig, client, s.T(), controllerCount, uint(s.ControllerCount+s.WorkerCount)), "While verifying konnectivity mesh")
	}

	s.T().Log("Testing that the load balancer is actually balancing the load")
	s.verifyLoadBalanced(ctx, lb, s.KubeAPIExternalPort, s.ControllerCount)
	if s.loadBalanceAllPorts {
		s.T().Log("Testing that the load balancer is balancing the k0s API port, too")
		s.verifyLoadBalanced(ctx, lb, s.K0sAPIExternalPort, s.ControllerCount)
	}
	s.checkPorts(ctx, lb, s.BalancedPorts(specs[s.loadBalanceAllPorts]))

	s.T().Log("Verify that controllers resolved the interface name from the MAC address correctly")
	for idx := range s.ControllerCount {
		keepalivedCfg := s.GetFileFromController(idx, "/run/k0s/keepalived.conf")
		s.Require().Contains(keepalivedCfg, "interface eth0", "Expected keepalived to resolve the interface name from the MAC address")
		if idx == 2 {
			s.Require().NotContains(keepalivedCfg, s.getMAC(idx), "Expected keepalived to have an interface configured")
		}
	}

	s.Run("the other way around", func() {
		allPorts := !s.loadBalanceAllPorts
		s.RestartControllersWithConfig("/tmp/k0s.yaml", func(idx int) string { return s.getK0sCfg(lb, s.nic(idx), specs[allPorts]) })
		// A controller holds the virtual IP again after the restarts.
		s.Require().EventuallyWithT(func(c *assert.CollectT) {
			assert.Len(c, common.VIPHolders(ctx, &s.BootlooseSuite, lb, s.isIPv6Only), 1, "Exactly one controller should hold the virtual IP")
		}, 2*time.Minute, time.Second)
		s.checkPorts(ctx, lb, s.BalancedPorts(specs[allPorts]))
		s.verifyLoadBalanced(ctx, lb, s.KubeAPIExternalPort, s.ControllerCount)
		if allPorts {
			s.verifyLoadBalanced(ctx, lb, s.K0sAPIExternalPort, s.ControllerCount)
		}
	})
}

// checkDummy checks that the dummy interface isn't present in the node.
func (s *CPLBUserSpaceSuite) checkDummy(ctx context.Context, node string) {
	ssh, err := s.SSH(ctx, node)
	s.Require().NoError(err)
	defer ssh.Disconnect()

	s.Require().Error(ssh.Exec(ctx, "ip --oneline addr show dummyvip0", common.SSHStreams{}))
}

// verifyLoadBalanced checks that the port on the virtual IP reaches the given number of controllers.
func (s *CPLBUserSpaceSuite) verifyLoadBalanced(ctx context.Context, lb string, port int, controllers int) {
	url := url.URL{Scheme: "https", Host: net.JoinHostPort(lb, strconv.Itoa(port))}
	s.Require().EventuallyWithT(func(c *assert.CollectT) {
		// Other stuff may be querying the controller, running the HTTPS request 15 times
		// should be more than we need. A single controller is still checked a few times.
		signatures := make(map[string]struct{})
		for attempt := 0; attempt < 15 && (len(signatures) < controllers || controllers == 1 && attempt < 5); attempt++ {
			signature, err := common.ServerCertSignature(ctx, url.String())
			if !assert.NoError(c, err) {
				return
			}
			signatures[signature] = struct{}{}
		}
		assert.Len(c, signatures, controllers, "Port %d on the virtual IP should reach %d controllers", port, controllers)
	}, 3*time.Minute, 2*time.Second)
}

// checkPorts checks that each controller redirects exactly the given ports on the virtual IP to their proxy ports,
// and that the userspace proxy listens on exactly those proxy ports.
func (s *CPLBUserSpaceSuite) checkPorts(ctx context.Context, lb string, ports []v1beta1.BalancedPort) {
	redirects := map[int]int{}
	for _, p := range ports {
		redirects[p.Port] = p.ProxyPort
	}
	wantRedirects := fmt.Sprint(redirects)
	wantListeners := fmt.Sprint(slices.Sorted(maps.Values(redirects)))
	iptables := "iptables"
	if s.isIPv6Only {
		iptables = "ip6tables"
	}
	redirect := regexp.MustCompile(`--dport (\d+) .*(?:--to-ports |--to-destination \S*:)(\d+)`)
	for idx := range s.ControllerCount {
		node := s.ControllerNode(idx)
		s.Require().EventuallyWithT(func(c *assert.CollectT) {
			ssh, err := s.SSH(ctx, node)
			if !assert.NoError(c, err) {
				return
			}
			defer ssh.Disconnect()

			// Ports redirected on the virtual IP, mapped to their target ports.
			rules, err := ssh.ExecWithOutput(ctx, "/var/lib/k0s/bin/"+iptables+" -t nat -S PREROUTING")
			if !assert.NoError(c, err) {
				return
			}
			got := map[int]int{}
			for rule := range strings.Lines(rules) {
				if m := redirect.FindStringSubmatch(rule); m != nil && strings.Contains(rule, "-d "+lb+"/") {
					port, _ := strconv.Atoi(m[1])
					got[port], _ = strconv.Atoi(m[2])
				}
			}
			assert.Equal(c, wantRedirects, fmt.Sprint(got), "Controller %s should redirect these ports on the virtual IP", node)

			// Proxy ports that the userspace proxy listens on.
			listening, err := ssh.ExecWithOutput(ctx, "netstat -tln")
			if !assert.NoError(c, err) {
				return
			}
			listeners := []int{}
			for _, p := range s.BalancedPorts(common.AllPortsKeepalivedSpec()) {
				if regexp.MustCompile(fmt.Sprintf(`:%d\s`, p.ProxyPort)).MatchString(listening) {
					listeners = append(listeners, p.ProxyPort)
				}
			}
			assert.Equal(c, wantListeners, fmt.Sprint(listeners), "The userspace proxy on controller %s should listen on these ports", node)
		}, 2*time.Minute, time.Second)
	}
}

// nic returns the interface setting of the controller, so that all NIC selection mechanisms are tested, the
// default, the interface name and the MAC address.
func (s *CPLBUserSpaceSuite) nic(idx int) string {
	switch idx {
	case 1:
		return "eth0"
	case 2:
		return s.getMAC(idx)
	}
	return ""
}

func (s *CPLBUserSpaceSuite) getMAC(idx int) string {
	return s.GetFileFromController(idx, "/sys/class/net/eth0/address")
}

// TestKeepAlivedSuite runs the keepalived test suite. It verifies that the
// virtual IP is working by joining a node to the cluster using the VIP.
func TestCPLBUserSpaceSuite(t *testing.T) {
	cplbSuite := &CPLBUserSpaceSuite{
		BootlooseSuite: common.BootlooseSuite{
			ControllerCount: 3,
			WorkerCount:     1,
		},
		// The IPVS suite covers IPv4 with only the API server port.
		loadBalanceAllPorts: true,
	}

	target := os.Getenv("K0S_INTTEST_TARGET")
	switch {
	case strings.Contains(target, "ipv6"):
		t.Log("Testing IPv6 only mode")
		cplbSuite.isIPv6Only = true
		cplbSuite.useExternalAddress = true
		cplbSuite.loadBalanceAllPorts = false
		cplbSuite.Networks = []string{"bridge-ipv6"}
		cplbSuite.AirgapImageBundleMountPoints = []string{"/var/lib/k0s/images/bundle-ipv6.tar"}

	case strings.Contains(target, "extaddr"):
		t.Log("Testing external address")
		cplbSuite.useExternalAddress = true
	}

	suite.Run(t, cplbSuite)

}
