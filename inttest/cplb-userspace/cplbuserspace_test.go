// SPDX-FileCopyrightText: 2024 k0s authors
// SPDX-License-Identifier: Apache-2.0

package keepalived

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/hex"
	"errors"
	"fmt"
	"html/template"
	"net"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"testing"

	"github.com/k0sproject/k0s/pkg/token"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/k0sproject/k0s/inttest/common"
	"github.com/stretchr/testify/suite"
)

type CPLBUserSpaceSuite struct {
	common.BootlooseSuite
	isIPv6Only         bool
	useExternalAddress bool
}

const nllbControllerConfig = `
spec:
  network:
    controlPlaneLoadBalancing:
      enabled: true
      type: Keepalived
      keepalived:
        vrrpInstances:
        - virtualIPs: ["%s"]
          authPass: "123456"
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
{{ if .IsIPv6Only }}
    podCIDR: fd00::/108
    serviceCIDR: fd01::/108
{{ end }}
`

func (s *CPLBUserSpaceSuite) getK0sCfg(lb string, nic string) string {
	if !s.useExternalAddress {
		return fmt.Sprintf(nllbControllerConfig, common.GetCPLBVIPCIDR(lb, s.isIPv6Only))
	}

	k0sCfg := bytes.NewBuffer([]byte{})
	data := map[string]any{
		"ExtAddr":    lb,
		"VIP":        common.GetCPLBVIPCIDR(lb, s.isIPv6Only),
		"IsIPv6Only": s.isIPv6Only,
		"Interface":  nic,
	}
	s.Require().NoError(template.Must(template.New("k0s.yaml").
		Parse(extAddrControllerConfig)).
		Execute(k0sCfg, data), "can't execute k0s.yaml template")
	return k0sCfg.String()
}

// SetupTest prepares the controller and filesystem, getting it into a consistent
// state which we can run tests against.
func (s *CPLBUserSpaceSuite) TestK0sGetsUp() {
	lb := common.GetCPLBVIP(&s.BootlooseSuite, s.isIPv6Only)

	ctx := s.Context()
	var joinToken string
	for idx := range s.ControllerCount {
		// Test that all NIC selection mechanisms, default, interface name and MAC address work
		nic := ""
		switch idx {
		case 1:
			nic = "eth0"
		case 2:
			nic = s.getMAC(ctx, s.ControllerNode(idx))
		}
		k0sCfg := s.getK0sCfg(lb, nic)
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

	client, err := s.KubeClient(s.ControllerNode(0))
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
	count := 0
	for idx := range s.ControllerCount {
		if s.hasVIP(ctx, s.ControllerNode(idx), lb) {
			count++
		}
	}
	s.Require().Equal(1, count, "Expected exactly one controller to have the VIP")

	// Verify that controller+worker nodes are working normally.
	s.T().Log("waiting to see CNI pods ready")
	if s.useExternalAddress {
		s.Require().NoError(common.WaitForDaemonSet(ctx, client, "calico-node", metav1.NamespaceSystem), "calico-node did not start")
	} else {
		s.Require().NoError(common.WaitForKubeRouterReady(ctx, client), "kube router did not start")
	}

	s.T().Log("waiting to see konnectivity-agent pods ready")
	s.Require().NoError(common.WaitForDaemonSet(ctx, client, "konnectivity-agent", metav1.NamespaceSystem), "konnectivity-agent did not start")
	s.T().Log("waiting to get logs from pods")
	s.Require().NoError(common.WaitForPodLogs(ctx, client, metav1.NamespaceSystem))

	s.T().Log("Testing that the load balancer is actually balancing the load")
	// Other stuff may be querying the controller, running the HTTPS request 15 times
	// should be more than we need.
	attempt := 0
	signatures := make(map[string]int)
	url := url.URL{Scheme: "https", Host: net.JoinHostPort(lb, strconv.Itoa(6443))}
	for len(signatures) < 3 {
		signature, err := getServerCertSignature(ctx, url.String())
		s.Require().NoError(err)
		signatures[signature] = 1
		attempt++
		s.Require().LessOrEqual(attempt, 15, "Failed to get a signature from all controllers")
	}

	s.T().Log("Verify that controllers resolved the interface name from the MAC address correctly")
	for idx := range s.ControllerCount {
		keepalivedCfg := s.getFile(ctx, s.ControllerNode(idx), "/run/k0s/keepalived.conf")
		s.Require().Contains(keepalivedCfg, "interface eth0", "Expected keepalived to resolve the interface name from the MAC address")
		if idx == 2 {
			s.Require().NotContains(keepalivedCfg, s.getMAC(ctx, s.ControllerNode(idx)), "Expected keepalived to have an interface configured")
		}
	}
}

// checkDummy checks that the dummy interface isn't present in the node.
func (s *CPLBUserSpaceSuite) checkDummy(ctx context.Context, node string) {
	ssh, err := s.SSH(ctx, node)
	s.Require().NoError(err)
	defer ssh.Disconnect()

	s.Require().Error(ssh.Exec(ctx, "ip --oneline addr show dummyvip0", common.SSHStreams{}))
}

// hasVIP checks that the dummy interface is present on the given node and
// that it has only the virtual IP address.
func (s *CPLBUserSpaceSuite) hasVIP(ctx context.Context, node string, vip string) bool {
	ssh, err := s.SSH(ctx, node)
	s.Require().NoError(err)
	defer ssh.Disconnect()

	output, err := ssh.ExecWithOutput(ctx, "ip --oneline addr show eth0")
	s.Require().NoError(err)

	return strings.Contains(output, fmt.Sprintf(" %s scope", common.GetCPLBVIPCIDR(vip, s.isIPv6Only)))
}

// getServerCertSignature connects to the given HTTPS URL and returns the server certificate signature.
func getServerCertSignature(ctx context.Context, url string) (string, error) {
	// Create a custom HTTP client with a custom TLS configuration
	client := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true, // Skip verification for demonstration purposes
			},
		},
	}
	defer client.CloseIdleConnections()

	// Make a request to the URL

	// Make a request to the URL with the context
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}

	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	// Get the TLS connection state
	connState := resp.TLS
	if connState == nil {
		return "", errors.New("no TLS connection state")
	}

	// Get the server certificate
	if len(connState.PeerCertificates) != 1 {
		return "", errors.New("no server certificate found")
	}
	cert := connState.PeerCertificates[0]

	// Get the certificate signature
	signature := cert.Signature

	// Return the signature as a hex string
	return hex.EncodeToString(signature), nil
}

func (s *CPLBUserSpaceSuite) getMAC(ctx context.Context, nodeName string) string {
	return s.getFile(ctx, nodeName, "/sys/class/net/eth0/address")
}

func (s *CPLBUserSpaceSuite) getFile(ctx context.Context, nodeName string, path string) string {
	ssh, err := s.SSH(ctx, nodeName)
	s.Require().NoError(err)
	defer ssh.Disconnect()

	output, err := ssh.ExecWithOutput(ctx, "cat "+path)
	s.Require().NoError(err)

	return output
}

// TestKeepAlivedSuite runs the keepalived test suite. It verifies that the
// virtual IP is working by joining a node to the cluster using the VIP.
func TestCPLBUserSpaceSuite(t *testing.T) {
	cplbSuite := &CPLBUserSpaceSuite{
		BootlooseSuite: common.BootlooseSuite{
			ControllerCount: 3,
			WorkerCount:     1,
		},
	}

	target := os.Getenv("K0S_INTTEST_TARGET")
	switch {
	case strings.Contains(target, "ipv6"):
		t.Log("Testing IPv6 only mode")
		cplbSuite.isIPv6Only = true
		cplbSuite.useExternalAddress = true
		cplbSuite.Networks = []string{"bridge-ipv6"}
		cplbSuite.AirgapImageBundleMountPoints = []string{"/var/lib/k0s/images/bundle-ipv6.tar"}

	case strings.Contains(target, "extaddr"):
		t.Log("Testing external address")
		cplbSuite.useExternalAddress = true
	}

	suite.Run(t, cplbSuite)

}
