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

import system "k8s.io/system-validators/validators"

func ReportSysinfo() []error {
	spec := sysinfoSpec()
	return spec.run(system.DefaultReporter)
}

func sysinfoSpec() K0sSpec {
	spec := preflightSpec()

	var optional []system.KernelConfig
	for _, config := range system.DefaultSysSpec.KernelSpec.Optional {
		switch config.Name {
		case "AUFS_FS":
			continue
		}
		optional = append(optional, config)
	}

	spec.sys.KernelSpec.Optional = append(optional, []system.KernelConfig{
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
	}...)

	return spec
}
