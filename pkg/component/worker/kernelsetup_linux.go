/*
Copyright 2020 k0s authors

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
	err := exec.Command("modprobe", module)
	if err != nil {
		logrus.Warnf("failed to load %s kernel module: %s", module, err)
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

// KernelMajorVersion returns the major version number of the running kernel
func KernelMajorVersion() byte {
	data, err := os.ReadFile("/proc/sys/kernel/osrelease")
	if err != nil {
		return 0
	}
	return data[0] - '0'
}
