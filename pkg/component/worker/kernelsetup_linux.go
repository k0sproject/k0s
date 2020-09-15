// +build linux

package worker

import (
	"io/ioutil"
	"os/exec"
	"path"
	"strings"

	"github.com/Mirantis/mke/pkg/util"
	"github.com/sirupsen/logrus"
)

// check if kernel has overlay fs
func hasFilesystem(filesystem string) bool {
	data, err := ioutil.ReadFile("/proc/filesystems")
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
	err := ioutil.WriteFile(file, []byte("1"), 0644)
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
	if !util.FileExists("/proc/net/nf_conntrack") {
		modprobe("nf_conntrack")
	}
	if !util.FileExists("/proc/sys/net/bridge/bridge-nf-call-iptables") {
		modprobe("br_netfilter")
	}
	enableSysCtl("net/ipv4/conf/all/forwarding")
	enableSysCtl("net/ipv4/conf/default/forwarding")
	enableSysCtl("net/ipv6/conf/all/forwarding")
	enableSysCtl("net/ipv6/conf/default/forwarding")
	enableSysCtl("net/bridge/bridge-nf-call-iptables")
	enableSysCtl("net/bridge/bridge-nf-call-ip6tables")
}
