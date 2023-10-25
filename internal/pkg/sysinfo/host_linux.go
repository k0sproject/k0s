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

package sysinfo

import (
	"regexp"

	"github.com/k0sproject/k0s/internal/pkg/sysinfo/probes"
	"github.com/k0sproject/k0s/internal/pkg/sysinfo/probes/linux"
)

func (s *K0sSysinfoSpec) addHostSpecificProbes(p probes.Probes) {

	linux := linux.RequireLinux(p)

	linux.AssertKernelRelease(func(release string) string {
		re := regexp.MustCompile(`^3\.1[0-9]|[4-9]|[1-9][0-9]`)
		if !re.MatchString(release) {
			return "expected 3.10 and above"
		}
		return ""
	})

	linux.AssertProcessMaxFileDescriptors(65536)
	linux.AssertAppArmor()

	if s.WorkerRoleEnabled {
		probes.AssertExecutableInPath(linux, "modprobe")
		probes.AssertExecutableInPath(linux, "mount")
		probes.AssertExecutableInPath(linux, "umount")
		linux.RequireProcFS()
		addCgroups(linux)
	}

	s.addKernelConfigs(linux)
}

func (s *K0sSysinfoSpec) addKernelConfigs(linux *linux.LinuxProbes) {
	// Most of the probes were added in k/k#32427 (k8s 1.5) without much
	// explanation. The probes here are documented as good as possible in
	// docs/external-runtime-deps.md (except for "debug" probes).

	//  Kernel config nesting is taken from the v4.3 kernel's menuconfig
	//  structure. If not stated otherwise, all comments are based on Linux 4.3,
	//  which is the first kernel release that supports all of those configs.

	if s.WorkerRoleEnabled {
		cgroups := linux.RequireKernelConfig("CGROUPS", "Control Group support")
		cgroups.RequireKernelConfig("CGROUP_FREEZER", "Freezer cgroup subsystem")
		cgroups.RequireKernelConfig("CGROUP_PIDS", "PIDs cgroup subsystem")
		cgroups.RequireKernelConfig("CGROUP_DEVICE", "Device controller for cgroups")
		cgroups.RequireKernelConfig("CPUSETS", "Cpuset support")
		cgroups.RequireKernelConfig("CGROUP_CPUACCT", "Simple CPU accounting cgroup subsystem")
		cgroups.RequireKernelConfig("MEMCG", "Memory Resource Controller for Control Groups")
		cgroups.AssertKernelConfig("CGROUP_HUGETLB", "HugeTLB Resource Controller for Control Groups")
		cgSched := cgroups.RequireKernelConfig("CGROUP_SCHED", "Group CPU scheduler")
		// https://github.com/kubernetes/kubeadm/issues/2335#issuecomment-717996215
		// > For reference https://github.com/torvalds/linux/blob/v4.3/kernel/sched/core.c#L8511-L8533
		// >
		// > - CONFIG_FAIR_GROUP_SCHED should be set as required, since there is
		// >   currently no way to disable using it in Kubernetes
		// > - CONFIG_CFS_BANDWIDTH should be set as optional, as long as
		// >   --cpu-cfs-quota=false actually works when CONFIG_CFS_BANDWIDTH=n
		fairGroupSched := cgSched.RequireKernelConfig("FAIR_GROUP_SCHED", "Group scheduling for SCHED_OTHER")
		fairGroupSched.AssertKernelConfig("CFS_BANDWIDTH", "CPU bandwidth provisioning for FAIR_GROUP_SCHED")
		cgroups.AssertKernelConfig("BLK_CGROUP", "Block IO controller")

		ns := linux.RequireKernelConfig("NAMESPACES", "Namespaces support")
		ns.RequireKernelConfig("UTS_NS", "UTS namespace")
		ns.RequireKernelConfig("IPC_NS", "IPC namespace")
		ns.RequireKernelConfig("PID_NS", "PID namespace")
		ns.RequireKernelConfig("NET_NS", "Network namespace")
	}

	net := linux.RequireKernelConfig("NET", "Networking support")
	inet := net.RequireKernelConfig("INET", "TCP/IP networking")

	if !s.WorkerRoleEnabled {
		return
	}

	netfilter := net.RequireKernelConfig("NETFILTER", "Network packet filtering framework (Netfilter)")

	// Prerequisite for required config NETFILTER_XT_MATCH_COMMENT
	netfilter.AssertKernelConfig("NETFILTER_ADVANCED", "Advanced netfilter configuration")

	// kube-proxy will fail without connection tracking
	netfilter.RequireKernelConfig("NF_CONNTRACK", "Netfilter connection tracking support")

	// Core Netfilter Configuration
	xtables := netfilter.RequireKernelConfig("NETFILTER_XTABLES", "Netfilter Xtables support")
	//  *** Xtables targets ***
	xtables.RequireKernelConfig("NETFILTER_XT_TARGET_REDIRECT", "REDIRECT target support", "IP_NF_TARGET_REDIRECT") // depends on NF_NAT
	//  *** Xtables matches ***
	xtables.RequireKernelConfig("NETFILTER_XT_MATCH_COMMENT", `"comment" match support`)

	// File systems
	linux.RequireKernelConfig("EXT4_FS", "The Extended 4 (ext4) filesystem")
	// Pseudo filesystems
	linux.RequireKernelConfig("PROC_FS", "/proc file system support")

	if !s.AddDebugProbes {
		return
	}

	inet.AssertKernelConfig("IPV6", "The IPv6 protocol")

	// It was found that the following modules were loaded automatically by
	// comparing the output of `lsmod` before and after running k0s.

	// Core Netfilter Configuration
	netfilter.AssertKernelConfig("NETFILTER_NETLINK", "")
	netfilter.AssertKernelConfig("NF_NAT", "") // prerequisite for some required configs
	//  *** Xtables combined modules ***
	xtables.AssertKernelConfig("NETFILTER_XT_MARK", "nfmark target and match support")
	xtables.AssertKernelConfig("NETFILTER_XT_SET", `set target and match support`)
	//  *** Xtables targets ***
	xtables.AssertKernelConfig("NETFILTER_XT_TARGET_MASQUERADE", "MASQUERADE target support",
		// This has been added in Linux 5.2 (adf82accc5f5), in v4.3 it was two modules:
		"IP_NF_TARGET_MASQUERADE", "IP6_NF_TARGET_MASQUERADE")
	xtables.AssertKernelConfig("NETFILTER_XT_NAT", `"SNAT and DNAT" targets support`)
	//  *** Xtables matches ***
	xtables.AssertKernelConfig("NETFILTER_XT_MATCH_ADDRTYPE", `"addrtype" address type match support`)
	xtables.AssertKernelConfig("NETFILTER_XT_MATCH_CONNTRACK", `"conntrack" connection tracking match support`)
	xtables.AssertKernelConfig("NETFILTER_XT_MATCH_MULTIPORT", `"multiport" Multiple port match support`)
	xtables.AssertKernelConfig("NETFILTER_XT_MATCH_RECENT", `"recent" match support`)
	xtables.AssertKernelConfig("NETFILTER_XT_MATCH_STATISTIC", `"statistic" match support`)

	ipSet := netfilter.AssertKernelConfig("IP_SET", "IP set support")
	ipSet.AssertKernelConfig("IP_SET_HASH_IP", "hash:ip set support")
	ipSet.AssertKernelConfig("IP_SET_HASH_NET", "hash:net set support")

	ipvs := netfilter.AssertKernelConfig("IP_VS", "IP virtual server support")
	ipvs.AssertKernelConfig("IP_VS_NFCT", "Netfilter connection tracking")
	ipvs.AssertKernelConfig("IP_VS_SH", "Source hashing scheduling")
	ipvs.AssertKernelConfig("IP_VS_RR", "Round-robin scheduling")
	ipvs.AssertKernelConfig("IP_VS_WRR", "Weighted round-robin scheduling")

	// IP: Netfilter Configuration
	netfilter.AssertKernelConfig("NF_CONNTRACK_IPV4", "IPv4 connetion tracking support (required for NAT)") // enables NF_NAT_IPV4, merged into NF_CONNTRACK in Linux 4.19 (a0ae2562c6c4)
	netfilter.AssertKernelConfig("NF_REJECT_IPV4", "IPv4 packet rejection")
	netfilter.AssertKernelConfig("NF_NAT_IPV4", "IPv4 NAT") // depends on NF_CONNTRACK_IPV4, selects NF_NAT, merged into NF_NAT in Linux 5.1 (3bf195ae6037)
	ipNFIPTables := netfilter.AssertKernelConfig("IP_NF_IPTABLES", "IP tables support")
	ipNFFilter := ipNFIPTables.AssertKernelConfig("IP_NF_FILTER", "Packet filtering")
	ipNFFilter.AssertKernelConfig("IP_NF_TARGET_REJECT", "REJECT target support")
	ipNFIPTables.AssertKernelConfig("IP_NF_NAT", "iptables NAT support") // selects NF_NAT
	ipNFIPTables.AssertKernelConfig("IP_NF_MANGLE", "Packet mangling")
	netfilter.AssertKernelConfig("NF_DEFRAG_IPV4", "")

	// IPv6: Netfilter Configuration
	netfilter.AssertKernelConfig("NF_CONNTRACK_IPV6", "IPv6 connetion tracking support (required for NAT)") // enables NF_NAT_IPV6, merged into NF_CONNTRACK in Linux 4.19 (a0ae2562c6c4)
	netfilter.AssertKernelConfig("NF_NAT_IPV6", "IPv6 NAT")                                                 // depends on NF_CONNTRACK_IPV6, selects NF_NAT, merged into NF_NAT in Linux 5.1 (3bf195ae6037)
	ip6NFIPTables := netfilter.AssertKernelConfig("IP6_NF_IPTABLES", "IP6 tables support")
	ip6NFIPTables.AssertKernelConfig("IP6_NF_FILTER", "Packet filtering")
	ip6NFIPTables.AssertKernelConfig("IP6_NF_MANGLE", "Packet mangling")
	ip6NFIPTables.AssertKernelConfig("IP6_NF_NAT", "ip6tables NAT support")
	netfilter.AssertKernelConfig("NF_DEFRAG_IPV6", "")

	bridge := net.AssertKernelConfig("BRIDGE", "802.1d Ethernet Bridging")
	bridge.AssertKernelConfig("LLC", "")
	bridge.AssertKernelConfig("STP", "")
}

func addCgroups(linux *linux.LinuxProbes) {
	cgroups := linux.RequireCgroups()
	cgroups.RequireControllers(
		"cpu",
		"cpuacct",
		"cpuset",
		"memory",
		"devices",
		"freezer",
		"pids",
	)
	cgroups.AssertControllers(
		"hugetlb",
		"blkio",
	)
}
