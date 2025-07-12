// SPDX-FileCopyrightText: 2020 k0s authors
// SPDX-License-Identifier: Apache-2.0

package worker

import (
	"os"
	"os/exec"
	"path"
	"strings"

	"github.com/k0sproject/k0s/internal/pkg/file"

	"github.com/sirupsen/logrus"
)

// check if kernel has overlay fs
func hasFilesystem(filesystem string) bool {
	data, err := os.ReadFile("/proc/filesystems")
	if err != nil {
		return false
	}
	for _, line := range strings.Split(string(data), "\n") {
		fields := strings.Fields(line)
		if len(fields) == 2 && fields[1] == filesystem {
			return true
		}
	}
	return false
}

func modprobe(module string) {
	out, err := exec.Command("modprobe", module).CombinedOutput()
	if err != nil {
		logrus.WithError(err).Warnf("failed to load kernel module %s: %s", module, out)
	}
}

func enableSysCtl(entry string) {
	file := path.Join("/proc", "sys", entry)
	err := os.WriteFile(file, []byte("1"), 0644)
	if err != nil {
		logrus.Warnf("Failed to enable %s: %s", file, err.Error())
	}
}

// KernelSetup sets the needed kernel tuning params. If setting the options fails, it only logs
// a warning but does not prevent the starting of worker
func KernelSetup() {
	if !hasFilesystem("overlay") {
		modprobe("overlay")
	}
	if !file.Exists("/proc/net/nf_conntrack") {
		modprobe("nf_conntrack")
	}
	if !file.Exists("/proc/sys/net/bridge/bridge-nf-call-iptables") {
		modprobe("br_netfilter")
	}
	// https://github.com/kubernetes/kubernetes/issues/108877
	if !file.Exists("/proc/net/ip_tables_targets") {
		modprobe("ip_tables")
	}
	enableSysCtl("net/ipv4/conf/all/forwarding")
	enableSysCtl("net/ipv4/conf/default/forwarding")
	enableSysCtl("net/ipv6/conf/all/forwarding")
	enableSysCtl("net/ipv6/conf/default/forwarding")
	enableSysCtl("net/bridge/bridge-nf-call-iptables")
	enableSysCtl("net/bridge/bridge-nf-call-ip6tables")
}
