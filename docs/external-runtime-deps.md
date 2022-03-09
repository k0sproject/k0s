# External runtime dependencies

k0s is packaged as a single binary, which includes all the needed components.
All the binaries are statically linked which means that in typical use cases
there's an absolute minimum of external runtime dependencies.

However, depending on the node role and cluster configuration, some of the
underlying components _may_ have specific dependencies, like OS level tools,
packages and libraries. This page aims to provide a comprehensive overview.

The following command checks for known requirements on a host (currently only
available on Linux):

```shell
k0s sysinfo
```

## A unique machine ID for multi-node setups

Whenever k0s is run in a multi-node setup (i.e. the `--single` command line flag
isn't used), k0s requires a [machine ID]: a unique host identifier that is
somewhat stable across reboots. For Linux, this ID is read from the files
`/var/lib/dbus/machine-id` or `/etc/machine-id`. For Windows, it's taken from
the registry key `HKEY_LOCAL_MACHINE\SOFTWARE\Microsoft\Cryptography\MachineGuid`.
If neither of the OS specific sources yield a result, k0s will fallback to use a
machine ID based on the hostname.

When running k0s on top of virtualized or containerized environments, you need
to [ensure][ensure-unique-id] that hosts get their own unique IDs, even if they
have been created from the same image.

[machine ID]: https://github.com/denisbrodbeck/machineid/blob/v1.0.1/README.md#what-you-get
[ensure-unique-id]: https://github.com/denisbrodbeck/machineid/blob/v1.0.1/README.md#unique-key-reliability

## Linux specific
<!--
This piece of documentation is best-effort and considered to be augmented and
extended in the future. The kernel and cgroups requirements are basically taken
from kubernetes/system-validators. Often there's no real hint as to why they are
required (although most requirements seem pretty obvious). Also need to check
for requirements of kube-router and calico.
-->

### Linux kernel configuration

Needless to say, as k0s operates Kubernetes worker nodes, there's a certain
number of needed Linux kernel modules and configurations that we need in the
system. This basically stems from the need to run both containers and also be
able to set up networking for the containers.

The needed kernel configuration items are listed below. All of them are
available in Kernel versions 4.3 and above. The list covers ONLY the
k0s/kubernetes components’ needs on worker nodes. Your own workloads may require
more.

- [CONFIG_INET](https://github.com/torvalds/linux/blob/v4.3/net/Kconfig#L5)
  - [CONFIG_NETFILTER_XT_TARGET_REDIRECT](https://github.com/torvalds/linux/blob/v4.3/net/netfilter/Kconfig#L853)
  - [CONFIG_NETFILTER_XT_MATCH_COMMENT](https://github.com/torvalds/linux/blob/v4.3/net/netfilter/Kconfig#L1002)
    (this used to be IP_NF_TARGET_REDIRECT before kernel version 3.7)

- [CONFIG_NAMESPACES](https://github.com/torvalds/linux/blob/v4.3/init/Kconfig#L1168)
  - [CONFIG_UTS_NS](https://github.com/torvalds/linux/blob/v4.3/init/Kconfig#L1180)
  - [CONFIG_IPC_NS](https://github.com/torvalds/linux/blob/v4.3/init/Kconfig#L1187)
  - [CONFIG_PID_NS](https://github.com/torvalds/linux/blob/v4.3/init/Kconfig#L1210)
  - [CONFIG_NET_NS](https://github.com/torvalds/linux/blob/v4.3/init/Kconfig#L1218)

- [CONFIG_CGROUPS](https://github.com/torvalds/linux/blob/v4.3/init/Kconfig#L927)
  - [CONFIG_CGROUP_FREEZER](https://github.com/torvalds/linux/blob/v4.3/init/Kconfig#L953)
  - [CONFIG_CGROUP_PIDS](https://github.com/torvalds/linux/blob/v4.3/init/Kconfig#L959)
  - [CONFIG_CGROUP_DEVICE](https://github.com/torvalds/linux/blob/v4.3/init/Kconfig#L975)
  - [CONFIG_CPUSETS](https://github.com/torvalds/linux/blob/v4.3/init/Kconfig#L981)
  - [CONFIG_CGROUP_CPUACCT](https://github.com/torvalds/linux/blob/v4.3/init/Kconfig#L996)
  - [CONFIG_MEMCG](https://github.com/torvalds/linux/blob/v4.3/init/Kconfig#L1005)
  - [CONFIG_CGROUP_SCHED](https://github.com/torvalds/linux/blob/v4.3/init/Kconfig#L1081)
    - [CONFIG_FAIR_GROUP_SCHED](https://github.com/torvalds/linux/blob/v4.3/init/Kconfig#L1090)

- [CONFIG_EXT4_FS](https://github.com/torvalds/linux/blob/v4.3/fs/ext4/Kconfig#L33)
- [CONFIG_PROC_FS](https://github.com/torvalds/linux/blob/v4.3/fs/proc/Kconfig#L1)

### cgroups

Required [cgroups] controllers:

- cpu
- cpuacct
- cpuset
- memory
- devices
- freezer
- pids

[cgroups]: https://man7.org/linux/man-pages/man7/cgroups.7.html

### containerd needs `apparmor_parser`

If containerd [detects][cd-aa] that the system is configured to use [AppArmor]
it will require a tool called `apparmor_parser` to be installed on the system.

[cd-aa]: https://github.com/containerd/containerd/blob/v1.5.10/pkg/apparmor/apparmor_linux.go#L37-L48
[AppArmor]: https://wiki.ubuntu.com/AppArmor

### Other dependencies in previous versions of k0s

- up until k0s v1.21.9+k0s.0: `iptables`  
  Required for worker nodes. Resolved by @ncopa in [#1046] by adding `iptables`
  and friends to k0s's embedded binaries.

- up until k0s v1.21.7+k0s.0: `find`, `du` and `nice`  
  Required for worker nodes. Resolved upstream by @ncopa in
  [kubernetes/kubernetes#96115], contained in Kubernetes 1.21.8 ([5b13c8f68d4])
  and 1.22.0 ([d45ba645a8f]).

[#1046]: https://github.com/k0sproject/k0s/pull/1046
[kubernetes/kubernetes#96115]: https://github.com/kubernetes/kubernetes/pull/96115
[5b13c8f68d4]: https://github.com/kubernetes/kubernetes/commit/5b13c8f68d48740261fa4c96ecb0a504982088a8
[d45ba645a8f]: https://github.com/kubernetes/kubernetes/commit/d45ba645a8f7b288284890a051c73bbae717da4b

## Windows specific
<!--
The kubernetes/system-validators require certain Windows versions starting with
Windows Server 2016. k0s states that it requires Windows Server 2019, though.
-->

TBD.
