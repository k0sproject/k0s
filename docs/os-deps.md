# OS dependencies

k0s is packaged as a single binary, which includes all the needed components. All the binaries are statically linked which means that in typical use cases there are no OS level dependencies.

However, some of the underlying components _may_ have dependencies on OS level tools and packages in certain circumstances. The known cases are documented below.

## Kernel configuration

Needless to say, as k0s operates Kubernetes there's a certain number of needed Linux kernel modules and configurations that we need in the system. This basically stems from the need to run both containers and also be able to set up networking for the containers.

The following command checks the kernel and the available modules from the host:

```shell
k0s sysinfo
```

The list of the needed kernel modules is shown below. The list covers ONLY the k0s/kubernetes components’ needs. Your own workload may require more.

```csv
CONFIG_NAMESPACES
CONFIG_NET_NS
CONFIG_PID_NS
CONFIG_IPC_NS
CONFIG_UTS_NS
CONFIG_CGROUPS
CONFIG_CGROUP_CPUACCT
CONFIG_CGROUP_DEVICE
CONFIG_CGROUP_FREEZER
CONFIG_CGROUP_PIDS
CONFIG_CGROUP_SCHED
CONFIG_CPUSETS
CONFIG_MEMCG
CONFIG_INET
CONFIG_EXT4_FS
CONFIG_PROC_FS
CONFIG_NETFILTER_XT_TARGET_REDIRECT
CONFIG_NETFILTER_XT_MATCH_COMMENT
CONFIG_FAIR_GROUP_SCHED
CONFIG_OVERLAY_FS
CONFIG_BLK_DEV_DM
CONFIG_CFS_BANDWIDTH
CONFIG_CGROUP_HUGETLB
CONFIG_SECCOMP
CONFIG_SECCOMP_FILTER
CONFIG_BRIDGE
CONFIG_IP6_NF_FILTER
CONFIG_IP6_NF_IPTABLES
CONFIG_IP6_NF_MANGLE
CONFIG_IP6_NF_NAT
CONFIG_IP_NF_FILTER
CONFIG_IP_NF_IPTABLES
CONFIG_IP_NF_MANGLE
CONFIG_IP_NF_NAT
CONFIG_IP_NF_TARGET_REJECT
CONFIG_IP_SET
CONFIG_IP_SET_HASH_IP
CONFIG_IP_SET_HASH_NET
CONFIG_IP_VS_NFCT
CONFIG_LLC
CONFIG_NETFILTER_NETLINK
CONFIG_NETFILTER_XTABLES
CONFIG_NETFILTER_XT_MARK
CONFIG_NETFILTER_XT_MATCH_ADDRTYPE
CONFIG_NETFILTER_XT_MATCH_CONNTRACK
CONFIG_NETFILTER_XT_MATCH_MULTIPORT
CONFIG_NETFILTER_XT_MATCH_RECENT
CONFIG_NETFILTER_XT_MATCH_STATISTIC
CONFIG_NETFILTER_XT_NAT
CONFIG_NETFILTER_XT_SET
CONFIG_NETFILTER_XT_TARGET_MASQUERADE
CONFIG_NF_CONNTRACK
CONFIG_NF_DEFRAG_IPV4
CONFIG_NF_DEFRAG_IPV6
CONFIG_NF_NAT
CONFIG_NF_REJECT_IPV4
CONFIG_STP
CGROUPS_CPU
CGROUPS_CPUACCT
CGROUPS_CPUSET
CGROUPS_DEVICES
CGROUPS_FREEZER
CGROUPS_MEMORY
CGROUPS_PIDS
CGROUPS_HUGETLB
```

## ContainerD needs `apparmor_parser`

If containerD [detects](https://github.com/containerd/containerd/blob/587fc092598791ab58bfa275958ce20cc5d80783/pkg/apparmor/apparmor_linux.go#L35-L44) that the system is configured to use [AppArmor](https://wiki.ubuntu.com/AppArmor) it will require a tool called `apparmor_parser` to be installed on the system.
