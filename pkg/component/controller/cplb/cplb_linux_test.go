// SPDX-FileCopyrightText: 2025 k0s authors
// SPDX-License-Identifier: Apache-2.0

package cplb

import (
	"strconv"
	"strings"
	"testing"
	"text/template"

	k0sAPI "github.com/k0sproject/k0s/pkg/apis/k0s/v1beta1"

	"github.com/stretchr/testify/assert"
)

const (
	kubeAPIPort           = 6443
	konnectivityAgentPort = 8132
)

func TestProxyRuleArgs(t *testing.T) {
	port := k0sAPI.BalancedPort{Port: konnectivityAgentPort, ProxyPort: 6445}
	for _, test := range []struct {
		name, forwarding, address, vip, target string
	}{
		{"DNAT by default", "", "10.0.0.2", "10.0.0.100", "-j DNAT --to-destination 10.0.0.2:6445"},
		{"DNAT on IPv6", "DNAT", "fd00::2", "fd00::100", "-j DNAT --to-destination [fd00::2]:6445"},
		{"REDIRECT", "REDIRECT", "10.0.0.2", "10.0.0.100", "-j REDIRECT --to-port 6445"},
		{"other IP family", "DNAT", "fd00::2", "10.0.0.100", "-j REDIRECT --to-port 6445"},
		{"no address", "DNAT", "", "10.0.0.100", "-j REDIRECT --to-port 6445"},
	} {
		t.Run(test.name, func(t *testing.T) {
			k := &Keepalived{
				Config:  &k0sAPI.KeepalivedSpec{UserSpaceProxyForwarding: k0sAPI.UserSpaceProxyForwarding(test.forwarding)},
				Address: test.address,
			}
			args := strings.Join(k.proxyRuleArgs("-A", test.vip, port), " ")
			assert.Equal(t, "-t nat -A PREROUTING -p tcp -d "+test.vip+" --dport 8132 "+test.target, args)
		})
	}
}

func TestEscapeSingleQuotes(t *testing.T) {
	assert.Equal(t, `It\'s a "nice" \'test\'`, escapeSingleQuotes(`It's a "nice" \'test'`))
}

func TestVirtualServersTemplate(t *testing.T) {
	render := func(ports ...int) string {
		var out strings.Builder
		templ := template.Must(template.New("vs").Parse(KeepalivedVirtualServersConfigTemplate))
		if !assert.NoError(t, templ.Execute(&out, &keepalivedConfig{
			VirtualServers:    []k0sAPI.VirtualServer{{IPAddress: "10.0.0.100", LBAlgo: "rr", LBKind: "DR"}},
			RealServers:       []string{"10.0.0.1", "10.0.0.2"},
			APIServerPort:     kubeAPIPort,
			LoadBalancedPorts: ports,
		})) {
			return ""
		}
		return out.String()
	}

	config := render(kubeAPIPort, konnectivityAgentPort)
	for _, port := range []string{strconv.Itoa(kubeAPIPort), strconv.Itoa(konnectivityAgentPort)} {
		assert.Contains(t, config, "virtual_server 10.0.0.100 "+port+" {", "each port should get a virtual server")
		assert.Contains(t, config, "real_server 10.0.0.2 "+port+" {", "each real server should be added for each port")
		assert.Contains(t, config, "connect_port "+port, "each port should be health checked")
	}
	assert.NotContains(t, render(kubeAPIPort), strconv.Itoa(konnectivityAgentPort), "ports that aren't load balanced should be left out")
}

func TestVRRPTemplateNotify(t *testing.T) {
	render := func(instances ...k0sAPI.VRRPInstance) string {
		var out strings.Builder
		templ := template.Must(template.New("vrrp").Parse(KeepalivedVRRPConfigTemplate))
		if !assert.NoError(t, templ.Execute(&out, &keepalivedConfig{
			VRRPInstances: instances, IPVSLoadBalancer: true, K0sBin: "/k0s", RunDir: "/run/k0s",
		})) {
			return ""
		}
		return out.String()
	}
	instance := k0sAPI.VRRPInstance{Interface: "eth0", VirtualIPs: []string{"10.0.0.100/24", "fd00::100/64"}}

	config := render(instance)
	for _, state := range []string{"MASTER", "BACKUP"} {
		assert.Contains(t, config, "keepalived-setstate -r '/run/k0s' -s "+state+" --interface 'eth0' --virtual-ip '10.0.0.100/24' --virtual-ip 'fd00::100/64'",
			"the %s script should get the interface and the virtual IPs to decide the state", state)
	}
	assert.NotContains(t, render(instance, instance), "keepalived-setstate", "several VRRP instances should have no state scripts")
}
