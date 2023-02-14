/*
Copyright 2022 k0s authors

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
	"context"
	"fmt"
	"net"
	"os"
	"os/exec"
	"strings"

	"go.uber.org/multierr"
)

type Airgap struct {
	SSH  func(ctx context.Context, node string) (*SSHConnection, error)
	Logf func(format string, args ...any)
}

func (a *Airgap) LockdownMachines(ctx context.Context, nodes ...string) error {
	blockIPv6 := true
	if err := tryBlockIPv6(); err != nil {
		a.Logf("Not blocking IPv6: %s", err.Error())
		blockIPv6 = false
	}

	cidrs, err := getPrivateCIDRs()
	if err != nil {
		return err
	}

	a.Logf("Allowed CIDRs: %v", cidrs)

	for _, node := range nodes {
		if err := a.airgapMachine(ctx, node, cidrs, blockIPv6); err != nil {
			return err
		}
	}

	return nil
}

func tryBlockIPv6() error {
	_, err := exec.LookPath("modprobe")
	if err != nil {
		return err
	}

	err = exec.Command("modprobe", "ip6table_filter").Run()
	if err != nil && os.Geteuid() != 0 {
		for _, cmd := range []string{"sudo", "doas"} {
			userErr := exec.Command(cmd, "-n", "modprobe", "ip6table_filter").Run()
			if userErr == nil {
				return nil
			}
			err = multierr.Append(err, userErr)
		}
	}

	return err
}

func getPrivateCIDRs() (string, error) {
	cidrs := []net.IPNet{
		{IP: net.IP{127, 0, 0, 0}, Mask: net.IPv4Mask(255, 0, 0, 0)},
		{IP: net.IP{10, 0, 0, 0}, Mask: net.IPv4Mask(255, 0, 0, 0)},
		{IP: net.IP{172, 16, 0, 0}, Mask: net.IPv4Mask(255, 240, 0, 0)},
		{IP: net.IP{192, 168, 0, 0}, Mask: net.IPv4Mask(255, 255, 0, 0)},
	}

	localAddrs, err := net.InterfaceAddrs()
	if err != nil {
		return "", err
	}

localAddrs:
	for _, a := range localAddrs {
		ipnet, ok := a.(*net.IPNet)
		if !ok {
			continue
		}

		ip := ipnet.IP.To4()
		if ip == nil {
			continue
		}

		for _, cidr := range cidrs {
			if cidr.Contains(ip) {
				continue localAddrs
			}
		}

		cidrs = append(cidrs, net.IPNet{
			IP:   ip,
			Mask: net.IPv4Mask(255, 255, 255, 255),
		})
	}

	var cidrStrings []string
	for _, cidr := range cidrs {
		cidrStrings = append(cidrStrings, cidr.String())
	}

	return strings.Join(cidrStrings, " "), nil
}

func (a *Airgap) airgapMachine(ctx context.Context, name, cidrs string, blockIPv6 bool) error {
	const airgapScript = `
		ip6tables='%s'
		apk add --no-cache iptables $ip6tables
		for cidr in %s; do
			iptables -A INPUT -s $cidr -j ACCEPT
			iptables -A OUTPUT -d $cidr -j ACCEPT
		done
		iptables -A INPUT -j REJECT
		iptables -A OUTPUT -j REJECT
		if [ -n "$ip6tables" ]; then
			ip6tables -A INPUT -j REJECT
			ip6tables -A OUTPUT -j REJECT
		fi
		if curl -v github.com 1>&2; then
			echo Internet connectivity not properly disrupted! Aborting ...
			exit 1
		fi
	`

	a.Logf("Airgapping %s", name)

	ssh, err := a.SSH(ctx, name)
	if err != nil {
		return err
	}
	defer ssh.Disconnect()

	var ip6tables string
	if blockIPv6 {
		ip6tables = "ip6tables"
	}

	return ssh.Exec(ctx, "sh -e -", SSHStreams{
		In: strings.NewReader(fmt.Sprintf(airgapScript, ip6tables, cidrs)),
	})
}
