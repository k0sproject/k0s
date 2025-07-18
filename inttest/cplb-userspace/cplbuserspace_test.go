// Copyright 2024 k0s authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package keepalived

import (
	"context"
	"crypto/tls"
	"encoding/hex"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/k0sproject/k0s/inttest/common"
	"github.com/k0sproject/k0s/pkg/token"

	"github.com/stretchr/testify/suite"
)

type CPLBUserSpaceSuite struct {
	common.BootlooseSuite
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
        - virtualIPs: ["%s/16"]
          authPass: "123456"
    nodeLocalLoadBalancing:
      enabled: true
      type: EnvoyProxy
`

const extAddrControllerConfig = `
spec:
  api:
    externalAddress: %s
  network:
    provider: calico
    controlPlaneLoadBalancing:
      enabled: true
      type: Keepalived
      keepalived:
        vrrpInstances:
        - virtualIPs: ["%s/16"]
          authPass: "123456"
`

// SetupTest prepares the controller and filesystem, getting it into a consistent
// state which we can run tests against.
func (s *CPLBUserSpaceSuite) TestK0sGetsUp() {
	lb := s.getLBAddress()
	ctx := s.Context()
	var joinToken string

	for idx := range s.BootlooseSuite.ControllerCount {
		s.Require().NoError(s.WaitForSSH(s.ControllerNode(idx), 2*time.Minute, 1*time.Second))
		if s.useExternalAddress {
			s.PutFile(s.ControllerNode(idx), "/tmp/k0s.yaml", fmt.Sprintf(extAddrControllerConfig, lb, lb))
			// Note that the token is intentionally empty for the first controller
			s.Require().NoError(s.InitController(idx, "--config=/tmp/k0s.yaml", "--disable-components=endpoint-reconciler", "--enable-worker", joinToken))
		} else {
			s.PutFile(s.ControllerNode(idx), "/tmp/k0s.yaml", fmt.Sprintf(nllbControllerConfig, lb))
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
	for idx := range s.BootlooseSuite.ControllerCount {
		s.Require().Len(s.GetMembers(idx), s.BootlooseSuite.ControllerCount)
	}

	// Create a worker join token
	workerJoinToken, err := s.GetJoinToken(token.RoleWorker)
	s.Require().NoError(err)

	// Start the workers using the join token
	s.Require().NoError(s.RunWorkersWithToken(workerJoinToken))

	client, err := s.KubeClient(s.ControllerNode(0))
	s.Require().NoError(err)

	for idx := range s.BootlooseSuite.ControllerCount {
		s.Require().NoError(s.WaitForNodeReady(s.ControllerNode(idx), client))
	}
	s.Require().NoError(s.WaitForNodeReady(s.WorkerNode(0), client))

	// Verify that none of the servers has the dummy interface
	for idx := range s.BootlooseSuite.ControllerCount {
		s.checkDummy(ctx, s.ControllerNode(idx))
	}

	// Verify that only one controller has the VIP in eth0
	count := 0
	for idx := range s.BootlooseSuite.ControllerCount {
		if s.hasVIP(ctx, s.ControllerNode(idx), lb) {
			count++
		}
	}
	s.Require().Equal(1, count, "Expected exactly one controller to have the VIP")

	// Verify that controller+worker nodes are working normally.
	s.T().Log("waiting to see CNI pods ready")
	if s.useExternalAddress {
		s.Require().NoError(common.WaitForDaemonSet(ctx, client, "calico-node", "kube-system"), "calico-node did not start")
	} else {
		s.Require().NoError(common.WaitForKubeRouterReady(ctx, client), "kube router did not start")
	}

	s.T().Log("waiting to see konnectivity-agent pods ready")
	s.Require().NoError(common.WaitForDaemonSet(ctx, client, "konnectivity-agent", "kube-system"), "konnectivity-agent did not start")
	s.T().Log("waiting to get logs from pods")
	s.Require().NoError(common.WaitForPodLogs(ctx, client, "kube-system"))

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
}

// getLBAddress returns the IP address of the controller 0 and it adds 100 to
// the last octet unless it's bigger or equal to 154, in which case it
// subtracts 100. Theoretically this could result in an invalid IP address.
// This is so that get a virtual IP in the same subnet as the controller.
func (s *CPLBUserSpaceSuite) getLBAddress() string {
	ip := s.GetIPAddress(s.ControllerNode(0))
	addr := net.ParseIP(ip)
	ipv4 := addr.To4()
	if ipv4 == nil {
		s.T().Fatalf("This test doesn't support IPv6: %q", ip)
	}

	if ipv4[3] >= 154 {
		ipv4[3] -= 100
	} else {
		ipv4[3] += 100
	}

	return ipv4.String()
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

	return strings.Contains(output, fmt.Sprintf("inet %s/16", vip))
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

	if err != nil {
		return "", err
	}

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
	case strings.Contains(target, "extaddr"):
		t.Log("Testing external address")
		cplbSuite.useExternalAddress = true
	}

	suite.Run(t, cplbSuite)

}
