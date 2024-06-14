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
	"bytes"
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"strings"
)

type Airgap struct {
	SSH  func(ctx context.Context, node string) (*SSHConnection, error)
	Logf func(format string, args ...any)
}

func (a *Airgap) LockdownMachines(ctx context.Context, nodes ...string) error {
	v4CIDRs, v6CIDRs, err := getPrivateCIDRs()
	if err != nil {
		return err
	}

	if err := tryBlockIPv6(); err != nil {
		a.Logf("Not blocking IPv6: %v", err)
		v6CIDRs = ""
	}

	a.Logf("Allowed CIDRs: %v %v", v4CIDRs, v6CIDRs)

	for _, node := range nodes {
		if err := a.airgapMachine(ctx, node, v4CIDRs, v6CIDRs); err != nil {
			return err
		}
	}

	return nil
}

func tryBlockIPv6() error {
	if initState, err := os.ReadFile("/sys/module/ip6table_filter/initstate"); err == nil {
		if bytes.Equal(bytes.TrimSpace(initState), []byte("live")) {
			return nil
		}
	}

	_, err := exec.LookPath("modprobe")
	if err != nil {
		return err
	}

	err = exec.Command("modprobe", "ip6table_filter").Run()
	if err != nil && os.Geteuid() != 0 {
		errs := []error{err}
		for _, cmd := range []string{"sudo", "doas"} {
			err := exec.Command(cmd, "-n", "modprobe", "ip6table_filter").Run()
			if err == nil {
				return nil
			}
			errs = append(errs, err)
		}
		err = errors.Join(errs...)
	}

	return err
}

func getPrivateCIDRs() (string, string, error) {
	v4CIDRs := []net.IPNet{
		{IP: net.IP{127, 0, 0, 0}, Mask: net.IPv4Mask(255, 0, 0, 0)},
		{IP: net.IP{10, 0, 0, 0}, Mask: net.IPv4Mask(255, 0, 0, 0)},
		{IP: net.IP{172, 16, 0, 0}, Mask: net.IPv4Mask(255, 240, 0, 0)},
		{IP: net.IP{192, 168, 0, 0}, Mask: net.IPv4Mask(255, 255, 0, 0)},
	}

	v6CIDRs := []net.IPNet{
		{ // Unique Local Addresses
			IP:   net.IP{0xfc, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0},
			Mask: net.CIDRMask(7, 8*net.IPv6len),
		},
		{ // Link-Local Addresses
			IP:   net.IP{0xfe, 0x80, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0},
			Mask: net.CIDRMask(10, 8*net.IPv6len),
		},
		{ // Loopback address
			IP:   net.IPv6loopback,
			Mask: net.CIDRMask(8*net.IPv6len, 8*net.IPv6len),
		},
	}

	localAddrs, err := net.InterfaceAddrs()
	if err != nil {
		return "", "", err
	}

localAddrs:
	for _, a := range localAddrs {
		ipnet, ok := a.(*net.IPNet)
		if !ok {
			continue
		}

		if ip := ipnet.IP.To4(); ip != nil {
			for _, cidr := range v4CIDRs {
				if cidr.Contains(ip) {
					continue localAddrs
				}
			}

			v4CIDRs = append(v4CIDRs, net.IPNet{
				IP:   ip,
				Mask: net.IPv4Mask(255, 255, 255, 255),
			})
		} else if ip := ipnet.IP.To16(); ip != nil {
			for _, cidr := range v6CIDRs {
				if cidr.Contains(ip) {
					continue localAddrs
				}
			}

			v6CIDRs = append(v6CIDRs, net.IPNet{
				IP:   ip,
				Mask: net.CIDRMask(8*net.IPv6len, 8*net.IPv6len),
			})
		}
	}

	var v4CIDRStrings, v6CIDRStrings []string
	for _, cidr := range v4CIDRs {
		v4CIDRStrings = append(v4CIDRStrings, cidr.String())
	}
	for _, cidr := range v6CIDRs {
		v6CIDRStrings = append(v6CIDRStrings, cidr.String())
	}

	return strings.Join(v4CIDRStrings, " "), strings.Join(v6CIDRStrings, " "), nil
}

func (a *Airgap) airgapMachine(ctx context.Context, name, v4CIDRs, v6CIDRs string) error {
	const airgapScript = `
		apk add --no-cache %s
		v4Cidrs='%s'
		v6Cidrs='%s'
		if [ -n "$v4Cidrs" ]; then
			for cidr in $v4Cidrs; do
				iptables -A INPUT -s $cidr -j ACCEPT
				iptables -A OUTPUT -d $cidr -j ACCEPT
			done
			iptables -A INPUT -j REJECT
			iptables -A OUTPUT -j REJECT
		fi

		if [ -n "$v6Cidrs" ]; then
			for cidr in $v6Cidrs; do
				ip6tables -A INPUT -s $cidr -j ACCEPT
				ip6tables -A OUTPUT -d $cidr -j ACCEPT
			done
			ip6tables -A INPUT -j REJECT
			ip6tables -A OUTPUT -j REJECT
		fi

		if curl -v github.com 1>&2; then
			echo Internet connectivity not properly disrupted! Aborting ...
			exit 1
		fi
	`

	var packages []string
	if v4CIDRs != "" {
		packages = append(packages, "iptables")
	}
	if v6CIDRs != "" {
		packages = append(packages, "ip6tables")
	}

	if len(packages) < 1 {
		return nil
	}

	a.Logf("Airgapping %s", name)

	ssh, err := a.SSH(ctx, name)
	if err != nil {
		return err
	}
	defer ssh.Disconnect()

	return ssh.Exec(ctx, "sh -e -", SSHStreams{
		In: strings.NewReader(fmt.Sprintf(airgapScript, strings.Join(packages, " "), v4CIDRs, v6CIDRs)),
	})
}
