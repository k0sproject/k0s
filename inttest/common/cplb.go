// SPDX-FileCopyrightText: 2026 k0s authors
// SPDX-License-Identifier: Apache-2.0

package common

import (
	"context"
	"crypto/tls"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/k0sproject/k0s/pkg/apis/k0s/v1beta1"

	"sigs.k8s.io/yaml"
)

// AllPortsKeepalivedSpec returns keepalived settings that also load balance the konnectivity agent and k0s API ports.
func AllPortsKeepalivedSpec() *v1beta1.KeepalivedSpec {
	return &v1beta1.KeepalivedSpec{LoadBalancedPorts: &v1beta1.LoadBalancedPorts{Konnectivity: new(true), K0sAPI: new(true)}}
}

// BalancedPorts returns the ports that the spec load balances, using the external ports of the cluster.
func (s *BootlooseSuite) BalancedPorts(spec *v1beta1.KeepalivedSpec) []v1beta1.BalancedPort {
	return spec.BalancedPorts(s.KubeAPIExternalPort, s.KonnectivityAgentPort, s.K0sAPIExternalPort)
}

// ControllerAddress returns the address of the controller in the IP family of the cluster.
func ControllerAddress(s *BootlooseSuite, idx int, vip string, isIPv6Only bool) string {
	if isIPv6Only {
		return FirstPublicIPv6Address(s, s.ControllerNode(idx), vip)
	}
	return s.GetIPAddress(s.ControllerNode(idx))
}

// VIPHolders returns the controllers that have the virtual IP on eth0. Unreachable controllers are skipped.
func VIPHolders(ctx context.Context, s *BootlooseSuite, vip string, isIPv6Only bool) []string {
	var holders []string
	for idx := range s.ControllerCount {
		node := s.ControllerNode(idx)
		ssh, err := s.SSH(ctx, node)
		if err != nil {
			continue
		}
		output, err := ssh.ExecWithOutput(ctx, "ip --oneline addr show eth0")
		ssh.Disconnect()
		if err == nil && strings.Contains(output, fmt.Sprintf(" %s scope", GetCPLBVIPCIDR(vip, isIPv6Only))) {
			holders = append(holders, node)
		}
	}
	return holders
}

// KeepalivedSettings renders the fields set in the spec as YAML, indented to fit the keepalived section of k0s.yaml.
func (s *BootlooseSuite) KeepalivedSettings(spec *v1beta1.KeepalivedSpec) string {
	data, err := yaml.Marshal(spec)
	s.Require().NoError(err)
	if strings.TrimSpace(string(data)) == "{}" {
		return ""
	}
	var settings strings.Builder
	for line := range strings.Lines(string(data)) {
		settings.WriteString("        " + line)
	}
	return settings.String()
}

// ServerCertSignature connects to the given HTTPS URL and returns the server certificate signature.
func ServerCertSignature(ctx context.Context, url string) (string, error) {
	client := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true, // Only the certificate is read, it isn't trusted
			},
		},
	}
	defer client.CloseIdleConnections()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}

	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	connState := resp.TLS
	if connState == nil {
		return "", errors.New("no TLS connection state")
	}

	if len(connState.PeerCertificates) != 1 {
		return "", errors.New("no server certificate found")
	}
	cert := connState.PeerCertificates[0]

	return hex.EncodeToString(cert.Signature), nil
}
