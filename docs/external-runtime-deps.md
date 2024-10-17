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

## Linux specific
<!--
This piece of documentation is best-effort and considered to be augmented and
extended in the future. The kernel and cgroup requirements are basically taken
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
available in Kernel versions 4.3 and above. If running on older kernels, check
if the distro in use has backported some features; nevertheless, it might meet
the requirements. k0s will check the Linux kernel release as part of its
pre-flight checks and issue a warning if it's below 3.10.

The list covers ONLY the k0s/kubernetes components’ needs on worker nodes. Your
own workloads may require more.

<!-- Kernel config nesting is taken from the v4.3 kernel's menuconfig structure. -->

- [`CONFIG_CGROUPS`](https://github.com/torvalds/linux/blob/v4.3/init/Kconfig#L927):
  Control Group support
  - [`CONFIG_CGROUP_FREEZER`](https://github.com/torvalds/linux/blob/v4.3/init/Kconfig#L953):
    Freezer cgroup subsystem
  - [`CONFIG_CGROUP_PIDS`](https://github.com/torvalds/linux/blob/v4.3/init/Kconfig#L959):
    PIDs cgroup subsystem  
    [kubernetes/kubeadm#2335 (comment)](https://github.com/kubernetes/kubeadm/issues/2335#issuecomment-722405527)
  - [`CONFIG_CGROUP_DEVICE`](https://github.com/torvalds/linux/blob/v4.3/init/Kconfig#L975):
    Device controller for cgroups
  - [`CONFIG_CPUSETS`](https://github.com/torvalds/linux/blob/v4.3/init/Kconfig#L981):
    Cpuset support
  - [`CONFIG_CGROUP_CPUACCT`](https://github.com/torvalds/linux/blob/v4.3/init/Kconfig#L996):
    Simple CPU accounting cgroup subsystem
  - [`CONFIG_MEMCG`](https://github.com/torvalds/linux/blob/v4.3/init/Kconfig#L1005):
    Memory Resource Controller for Control Groups
  - _(optional)_ [`CONFIG_CGROUP_HUGETLB`](https://github.com/torvalds/linux/blob/v4.3/init/Kconfig#L1055):
    HugeTLB Resource Controller for Control Groups  
    [kubernetes/kubeadm#2335 (comment)](https://github.com/kubernetes/kubeadm/issues/2335#issuecomment-722405527)
  - [`CONFIG_CGROUP_SCHED`](https://github.com/torvalds/linux/blob/v4.3/init/Kconfig#L1081):
    Group CPU scheduler
    - [`CONFIG_FAIR_GROUP_SCHED`](https://github.com/torvalds/linux/blob/v4.3/init/Kconfig#L1090):
      Group scheduling for SCHED_OTHER  
      [kubernetes/kubeadm#2335 (comment)](https://github.com/kubernetes/kubeadm/issues/2335#issuecomment-717996215)
      - _(optional)_ [`CONFIG_CFS_BANDWIDTH`](https://github.com/torvalds/linux/blob/v4.3/init/Kconfig#L1095):
        CPU bandwidth provisioning for FAIR_GROUP_SCHED  
        Required if CPU CFS quota enforcement is enabled for containers that
        specify CPU limits (`--cpu-cfs-quota`).
  - _(optional)_ [`CONFIG_BLK_CGROUP`](https://github.com/torvalds/linux/blob/v4.3/init/Kconfig#L1119):
    Block IO controller  
    [kubernetes/kubernetes#92287 (comment)](https://github.com/kubernetes/kubernetes/issues/92287#issuecomment-1010723587)
- [`CONFIG_NAMESPACES`](https://github.com/torvalds/linux/blob/v4.3/init/Kconfig#L1168):
  Namespaces support
  - [`CONFIG_UTS_NS`](https://github.com/torvalds/linux/blob/v4.3/init/Kconfig#L1180):
    UTS namespace
  - [`CONFIG_IPC_NS`](https://github.com/torvalds/linux/blob/v4.3/init/Kconfig#L1187):
    IPC namespace
  - [`CONFIG_PID_NS`](https://github.com/torvalds/linux/blob/v4.3/init/Kconfig#L1210):
    PID namespace
  - [`CONFIG_NET_NS`](https://github.com/torvalds/linux/blob/v4.3/init/Kconfig#L1218):
    Network namespace
- [`CONFIG_NET`](https://github.com/torvalds/linux/blob/v4.3/net/Kconfig#L5):
  Networking support
  - [`CONFIG_INET`](https://github.com/torvalds/linux/blob/v4.3/net/Kconfig#L58):
    TCP/IP networking
  - [`CONFIG_NETFILTER`](https://github.com/torvalds/linux/blob/v4.3/net/Kconfig#L109):
    Network packet filtering framework (Netfilter)
    - _(optional)_ [`CONFIG_NETFILTER_ADVANCED`](https://github.com/torvalds/linux/blob/v4.3/net/Kconfig#L171):
      Advanced netfilter configuration
    - [`CONFIG_NETFILTER_XTABLES`](https://github.com/torvalds/linux/blob/v4.3/net/netfilter/Kconfig#L567):
      Netfilter Xtables support
      - [`CONFIG_NETFILTER_XT_TARGET_REDIRECT`](https://github.com/torvalds/linux/blob/v4.3/net/netfilter/Kconfig#L853):
        REDIRECT target support
      - [`CONFIG_NETFILTER_XT_MATCH_COMMENT`](https://github.com/torvalds/linux/blob/v4.3/net/netfilter/Kconfig#L1002):
        "comment" match support
- [`CONFIG_EXT4_FS`](https://github.com/torvalds/linux/blob/v4.3/fs/ext4/Kconfig#L33):
  The Extended 4 (ext4) filesystem
- [`CONFIG_PROC_FS`](https://github.com/torvalds/linux/blob/v4.3/fs/proc/Kconfig#L1):
  /proc file system support

**Note:** As part of its pre-flight checks, k0s will try to inspect and validate
the kernel configuration. In order for that to succeed, the configuration needs
to be accessible at runtime. There are some typical places that k0s will check.
A bullet-proof way to ensure the accessibility is to enable
[`CONFIG_IKCONFIG_PROC`](https://github.com/torvalds/linux/blob/v4.3/init/Kconfig#L807),
and, if enabled as a module, to load the `configs` module: `modprobe configs`.

### Control Groups (cgroups)

Both [cgroup v1] and [cgroup v2] are supported.

Required [cgroup] controllers:

- cpu
- cpuacct
- cpuset
- memory
- devices
- freezer
- pids

Optional cgroup controllers:

- hugetlb ([kubernetes/kubeadm#2335 (comment)](https://github.com/kubernetes/kubeadm/issues/2335#issuecomment-722405527))
- blkio ([kubernetes/kubernetes#92287 (comment)](https://github.com/kubernetes/kubernetes/issues/92287#issuecomment-1010723587))  
   containerd and cri-o will use blkio to track disk I/O and throttling in both
   cgroup v1 and v2.

[cgroup]: https://man7.org/linux/man-pages/man7/cgroups.7.html
[cgroup v1]: https://www.kernel.org/doc/html/v5.16/admin-guide/cgroup-v1/
[cgroup v2]: https://www.kernel.org/doc/html/v5.16/admin-guide/cgroup-v2.html

### No integration with Name Service Switch (NSS) APIs

The k0s Linux binaries are by default statically linked against [musl libc].
This includes the binaries distributed on the GitHub releases pages. Static
linking ensures that k0s can run seamlessly across a wide range of Linux
environments by not requiring a specific standard C library to be installed on
the host system. However, this design choice means that k0s cannot use [glibc's
NSS APIs], which require dynamic linking.

This limitation is particularly relevant when a system uses NSS plugins, such as
[nss-myhostname], for resolving network names like `localhost`. Systems lacking
a dedicated stub resolver capable of handling `localhost` DNS queries
specifically will encounter issues running k0s. To mitigate this, users are
advised to either activate a stub DNS resolver, such as `systemd-resolved`, or
to manually add `localhost` entries to the `/etc/hosts` file as shown below:

```text
127.0.0.1 localhost
::1 localhost
```

[musl libc]: https://musl.libc.org/
[glibc's NSS APIs]: https://www.gnu.org/software/libc/manual/html_node/Name-Service-Switch.html
[nss-myhostname]: https://www.freedesktop.org/software/systemd/man/latest/nss-myhostname.html

### External hard dependencies

There are very few external tools that are needed or used.

#### mount/umount

When setting up pods, kubelet will call `mount` binary on the host. Similarly when destroying pods it will call `umount`. `mount` and `umount` are only needed on worker nodes where kubelet runs.

### External soft dependencies

There are a few external tools that may be needed or used under specific
circumstances:

#### containerd and AppArmor

In order to use containerd in conjunction with [AppArmor], it must be enabled in
the kernel and the `/sbin/apparmor_parser` executable must be installed on the
host, otherwise containerd will [disable][cd-aa] AppArmor support.

[cd-aa]: https://github.com/containerd/containerd/blob/v1.7.23/pkg/apparmor/apparmor_linux.go#L34-L45
[AppArmor]: https://wiki.ubuntu.com/AppArmor

#### iptables

iptables may be executed to detect if there are any existing iptables rules and
if those are in legacy of nft mode. If iptables is not found, k0s will assume
that there are no pre-existing iptables rules.

#### useradd / adduser

During `k0s install` the external tool `useradd` will be used on the controllers
to create system user accounts for k0s. If this does exist it will fall-back to
busybox's `adduser`.

#### userdel / deluser

`k0s reset` will execute either `userdel` or `deluser` to clean up system user
accounts.

#### modprobe

On k0s worker `modprobe` will be executed to load missing kernel modules if they
are not detected.

#### id

External `id` will be executed as a fallback if local user lookup fails, in case
NSS is used.

## Windows specific
<!--
The kubernetes/system-validators require certain Windows versions starting with
Windows Server 2016. k0s states that it requires Windows Server 2019, though.
-->

TBD.
