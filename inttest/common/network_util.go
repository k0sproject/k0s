/*
Copyright 2025 k0s authors

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

package common

import (
	"net"
	"strings"
)

// FirstPublicIPv6Address retrieves the first public IPv6 address from the eth0 interface of a node.
func FirstPublicIPv6Address(s *BootlooseSuite, nodeName string) string {
	ssh, err := s.SSH(s.Context(), nodeName)
	s.Require().NoError(err)
	defer ssh.Disconnect()

	output, err := ssh.ExecWithOutput(s.Context(), "ip -6 -oneline addr show eth0 scope global")
	s.Require().NoError(err)

	// Parse the output line by line
	for line := range strings.SplitSeq(strings.TrimSpace(output), "\n") {
		fields := strings.Fields(line)
		s.Require().GreaterOrEqual(len(fields), 4, "Expected at least 4 fields in the output line")

		ip, _, err := net.ParseCIDR(fields[3])
		s.Require().NoError(err, "Failed to parse IP address from output line")

		return ip.String()
	}

	s.Require().Fail("No IPv6 address found on eth0")
	return ""
}

// GetCPLBVIP returns the IP address of the controller 0 and it adds 100 to
// the last octet unless it's bigger or equal to 154, in which case it
// subtracts 100. Theoretically this could result in an invalid IP address.
// This is so that get a virtual IP in the same subnet as the controller.
func GetCPLBVIP(s *BootlooseSuite, isIPv6Only bool) string {
	var ip string
	if isIPv6Only {
		ip = FirstPublicIPv6Address(s, s.ControllerNode(0))
	} else {
		ip = s.GetIPAddress(s.ControllerNode(0))
	}

	addr := net.ParseIP(ip)
	if addr[15] >= 154 {
		addr[15] -= 100
	} else {
		addr[15] += 100
	}

	return addr.String()
}
