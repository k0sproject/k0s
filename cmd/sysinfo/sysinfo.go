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
	"runtime"

	"github.com/spf13/cobra"
	system "k8s.io/system-validators/validators"
)

func NewSysinfoCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "sysinfo",
		Short: "Display system information",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runSysinfo()
		},
	}

}

func runSysinfo() error {
	reporter := system.DefaultReporter

	var validators = []system.Validator{
		&system.KernelValidator{Reporter: reporter},
	}

	if runtime.GOOS == "linux" {
		validators = append(validators,
			&system.OSValidator{Reporter: reporter},
			&system.CgroupsValidator{Reporter: reporter},
		)
	}

	spec := system.DefaultSysSpec
	spec.KernelSpec.Required =
		// this is documented in docs/external-runtime-deps.md
		[]system.KernelConfig{
			{Name: "INET"},
			{Name: "NETFILTER_XT_TARGET_REDIRECT", Aliases: []string{"IP_NF_TARGET_REDIRECT"}},
			{Name: "NETFILTER_XT_MATCH_COMMENT"},
			{Name: "NAMESPACES"},
			{Name: "UTS_NS"},
			{Name: "IPC_NS"},
			{Name: "PID_NS"},
			{Name: "NET_NS"},
			{Name: "CGROUPS"},
			{Name: "CGROUP_FREEZER"},
			{Name: "CGROUP_PIDS"},
			{Name: "CGROUP_DEVICE"},
			{Name: "CPUSETS"},
			{Name: "CGROUP_CPUACCT"},
			{Name: "MEMCG"},
			{Name: "CGROUP_SCHED"},
			{Name: "FAIR_GROUP_SCHED"},
			{Name: "EXT4_FS"},
			{Name: "PROC_FS"},
		}

	spec.KernelSpec.Optional =
		[]system.KernelConfig{
			{Name: "OVERLAY_FS", Aliases: []string{"OVERLAYFS_FS"}, Description: "Required for overlayfs."},
			{Name: "BLK_DEV_DM", Description: "Required for devicemapper."},
			{Name: "CFS_BANDWIDTH", Description: "Required for CPU quota."},
			{Name: "CGROUP_HUGETLB", Description: "Required for hugetlb cgroup."},
			{Name: "SECCOMP", Description: "Required for seccomp."},
			{Name: "SECCOMP_FILTER", Description: "Required for seccomp mode 2."},

			{Name: "BRIDGE"},
			{Name: "IP6_NF_FILTER"},
			{Name: "IP6_NF_IPTABLES"},
			{Name: "IP6_NF_MANGLE"},
			{Name: "IP6_NF_NAT"},
			{Name: "IP_NF_FILTER"},
			{Name: "IP_NF_IPTABLES"},
			{Name: "IP_NF_MANGLE"},
			{Name: "IP_NF_NAT"},
			{Name: "IP_NF_TARGET_REJECT"},
			{Name: "IP_SET"},
			{Name: "IP_SET_HASH_IP"},
			{Name: "IP_SET_HASH_NET"},
			{Name: "IP_VS_NFCT"},
			{Name: "LLC"},
			{Name: "NETFILTER_NETLINK"},
			{Name: "NETFILTER_XTABLES"},
			{Name: "NETFILTER_XT_MARK"},
			{Name: "NETFILTER_XT_MATCH_ADDRTYPE"},
			{Name: "NETFILTER_XT_MATCH_CONNTRACK"},
			{Name: "NETFILTER_XT_MATCH_MULTIPORT"},
			{Name: "NETFILTER_XT_MATCH_RECENT"},
			{Name: "NETFILTER_XT_MATCH_STATISTIC"},
			{Name: "NETFILTER_XT_NAT"},
			{Name: "NETFILTER_XT_SET"},
			{Name: "NETFILTER_XT_TARGET_MASQUERADE"},
			{Name: "NF_CONNTRACK"},
			{Name: "NF_DEFRAG_IPV4"},
			{Name: "NF_DEFRAG_IPV6"},
			{Name: "NF_NAT"},
			{Name: "NF_REJECT_IPV4"},
			{Name: "STP"},
		}

	for _, v := range validators {
		_, _ = v.Validate(spec)
	}

	return nil
}
