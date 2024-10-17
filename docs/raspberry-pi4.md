# Create a Raspberry Pi 4 cluster

## Prerequisites

This guide assumes that you use a [Raspberry Pi 4 Model B] computer and a
sufficiently large SD card of at least 32 GB. We will be using [Ubuntu Linux]
for this guide, although k0s should run quite fine on other 64-bit Linux
distributions for the Raspberry Pi as well. Please [file a Bug] if you encounter
any obstacles.

[Raspberry Pi 4 Model B]: https://www.raspberrypi.org/products/raspberry-pi-4-model-b/
[Ubuntu Linux]: https://ubuntu.com/
[file a Bug]: https://github.com/k0sproject/k0s/issues/new?assignees=&labels=bug&template=BUG_REPORT.yml

## Set up the system

### Prepare SD card and boot up the Raspberry Pi

Install [Ubuntu Server 22.04.1 LTS 64-bit for Raspberry Pi][ubuntu-dl]. Ubuntu
provides a [step by step guide][ubuntu-pi] for the installation process. They
use [Raspberry Pi Imager], a specialized imaging utility that you can use to
write the Ubuntu image, amongst others, to your SD cards. Follow that
[guide][ubuntu-pi] to get a working installation. (You can skip part 5 of the
guide, since we won't need a Desktop Environment to run k0s.)

Alternatively, you can also opt to [download][2204-dl] the Ubuntu server image
for Raspberry Pi manually and write it to an SD card using a tool like `dd`:

```console
wget https://cdimage.ubuntu.com/releases/22.04.1/release/ubuntu-22.04.1-preinstalled-server-arm64+raspi.img.xz
unxz ubuntu-22.04.1-preinstalled-server-arm64+raspi.img.xz
dd if=ubuntu-22.04.1-preinstalled-server-arm64+raspi.img of=/dev/mmcblk0 bs=4M status=progress
```

**Note**: The manual process is more prone to accidental data loss than the
guided one via Raspberry Pi Imager. Be sure to choose the correct device names.
The previous content of the SD card will be wiped. Moreover, the partition
written to the SD card needs to be resized to make the full capacity of the card
available to Ubuntu. This can be achieved, for example, in this way:

```console
growpart /dev/mmcblk0 2
resize2fs /dev/mmcblk0p2
```

Ubuntu uses [cloud-init] to allow for automated customizations of the system
configuration. The cloud-init configuration files are located on the boot
partition of the SD card. You can mount that partition and modify those, e.g. to
provision network configuration, users, authorized SSH keys, additional packages
and also an automatic installation of k0s.

After you have prepared the SD card, plug it into the Raspberry Pi and boot it
up. Once cloud-init finished bootstrapping the system, the default login
credentials are set to user `ubuntu` with password `ubuntu` (which you will be
prompted to change on first login).

[ubuntu-dl]: https://ubuntu.com/download/raspberry-pi
[ubuntu-pi]: https://ubuntu.com/tutorials/how-to-install-ubuntu-on-your-raspberry-pi
[Raspberry Pi Imager]: https://www.raspberrypi.org/blog/raspberry-pi-imager-imaging-utility/
[2204-dl]: https://ubuntu.com/download/raspberry-pi/thank-you?version=22.04.1&architecture=server-arm64+raspi
[cloud-init]: https://cloud-init.io/

### Review network configurations

**Note**: For network configuration purposes, this documentation assumes that all
of your computers are connected on the same subnet.

Review k0s's [required ports and protocols] to ensure that your network and
firewall configurations allow necessary traffic for the cluster.

Review the [Ubuntu Server Networking Configuration] documentation to ensure that
all systems have a static IP address on the network, or that the network is
providing a static DHCP lease for the nodes. If the network should be managed
via cloud-init, please refer to [their documentation][cloud-init-network].

[required ports and protocols]: networking.md#required-ports-and-protocols
[Ubuntu Server Networking Configuration]: https://ubuntu.com/server/docs/network-configuration
[cloud-init-network]: https://cloudinit.readthedocs.io/en/latest/topics/network-config-format-v2.html#examples

### (Optional) Provision SSH keys

Ubuntu Server deploys and enables [OpenSSH](https://www.openssh.com/) via
cloud-init by default. Confirm, though, that for whichever user you will deploy
the cluster with on the build system, their SSH Key is [copied to each node's
root user][copy-ssh-key]. Before you start, the configuration should be such
that the current user can run:

```shell
ssh root@${HOST}
```

Where `${HOST}` is any node and the login can succeed with no further prompts.

[copy-ssh-key]: https://www.cyberciti.biz/faq/use-ssh-copy-id-with-an-openssh-server-listing-on-a-different-port/

### (Optional) Create a swap file

While having a swap file is _technically optional_, it can help to ease memory
pressure when running memory intensive workloads or on Raspberry Pis with less
than 8 GB of RAM.

1. To create a swap file:

    ```shell
    fallocate -l 2G /swapfile && \
    chmod 0600 /swapfile && \
    mkswap /swapfile && \
    swapon -a
    ```

2. Ensure that the usage of swap is not too aggressive by setting the `sudo
   sysctl vm.swappiness=10` (the default is generally higher) and configuring it
   to be persistent in `/etc/sysctl.d/*`.

3. Ensure that your swap is mounted after reboots by confirming that the
   following line exists in your `/etc/fstab` configuration:

    ```shell
    /swapfile         none           swap sw       0 0
    ```

## Download k0s

Download a [k0s release](https://github.com/k0sproject/k0s/releases/latest). For
example:

```shell
wget -O /tmp/k0s https://github.com/k0sproject/k0s/releases/download/v{{{ extra.k8s_version }}}+k0s.0/k0s-v{{{ extra.k8s_version }}}+k0s.0-arm64 # replace version number!
sudo install /tmp/k0s /usr/local/bin/k0s
```

― or ―

Use the k0s download script (as one command) to download the latest stable k0s
and make it executable in `/usr/bin/k0s`.

```shell
curl --proto '=https' --tlsv1.2 -sSf https://get.k0s.sh | sudo sh
```

At this point you can run `k0s`:

```console
ubuntu@ubuntu:~$ k0s version
v{{{ extra.k8s_version }}}+k0s.0
```

To check if k0s's [system requirements](system-requirements.md) and [external
runtime dependencies](external-runtime-deps.md) are fulfilled by your current
setup, you can invoke `k0s sysinfo`:

```console
ubuntu@ubuntu:~$ k0s sysinfo
Total memory: 3.7 GiB (pass)
File system of /var/lib: ext4 (pass)
Disk space available for /var/lib/k0s: 83.6 GiB (pass)
Relative disk space available for /var/lib/k0s: 20% (pass)
Operating system: Linux (pass)
  Linux kernel release: 5.15.0-1013-raspi (pass)
  Max. file descriptors per process: current: 1024 / max: 1048576 (warning: < 65536)
  AppArmor: unavailable (pass)
  Executable in PATH: modprobe: /usr/sbin/modprobe (pass)
  Executable in PATH: mount: /usr/bin/mount (pass)
  Executable in PATH: umount: /usr/bin/umount (pass)
  /proc file system: mounted (0x9fa0) (pass)
  Control Groups: version 2 (pass)
    cgroup controller "cpu": available (pass)
    cgroup controller "cpuacct": available (via cpu in version 2) (pass)
    cgroup controller "cpuset": available (pass)
    cgroup controller "memory": available (pass)
    cgroup controller "devices": unknown (warning: insufficient permissions, try with elevated permissions)
    cgroup controller "freezer": available (cgroup.freeze exists) (pass)
    cgroup controller "pids": available (pass)
    cgroup controller "hugetlb": available (pass)
    cgroup controller "blkio": available (via io in version 2) (pass)
  CONFIG_CGROUPS: Control Group support: built-in (pass)
    CONFIG_CGROUP_FREEZER: Freezer cgroup subsystem: built-in (pass)
    CONFIG_CGROUP_PIDS: PIDs cgroup subsystem: built-in (pass)
    CONFIG_CGROUP_DEVICE: Device controller for cgroups: built-in (pass)
    CONFIG_CPUSETS: Cpuset support: built-in (pass)
    CONFIG_CGROUP_CPUACCT: Simple CPU accounting cgroup subsystem: built-in (pass)
    CONFIG_MEMCG: Memory Resource Controller for Control Groups: built-in (pass)
    CONFIG_CGROUP_HUGETLB: HugeTLB Resource Controller for Control Groups: built-in (pass)
    CONFIG_CGROUP_SCHED: Group CPU scheduler: built-in (pass)
      CONFIG_FAIR_GROUP_SCHED: Group scheduling for SCHED_OTHER: built-in (pass)
        CONFIG_CFS_BANDWIDTH: CPU bandwidth provisioning for FAIR_GROUP_SCHED: built-in (pass)
    CONFIG_BLK_CGROUP: Block IO controller: built-in (pass)
  CONFIG_NAMESPACES: Namespaces support: built-in (pass)
    CONFIG_UTS_NS: UTS namespace: built-in (pass)
    CONFIG_IPC_NS: IPC namespace: built-in (pass)
    CONFIG_PID_NS: PID namespace: built-in (pass)
    CONFIG_NET_NS: Network namespace: built-in (pass)
  CONFIG_NET: Networking support: built-in (pass)
    CONFIG_INET: TCP/IP networking: built-in (pass)
      CONFIG_IPV6: The IPv6 protocol: built-in (pass)
    CONFIG_NETFILTER: Network packet filtering framework (Netfilter): built-in (pass)
      CONFIG_NETFILTER_ADVANCED: Advanced netfilter configuration: built-in (pass)
      CONFIG_NF_CONNTRACK: Netfilter connection tracking support: module (pass)
      CONFIG_NETFILTER_XTABLES: Netfilter Xtables support: module (pass)
        CONFIG_NETFILTER_XT_TARGET_REDIRECT: REDIRECT target support: module (pass)
        CONFIG_NETFILTER_XT_MATCH_COMMENT: "comment" match support: module (pass)
        CONFIG_NETFILTER_XT_MARK: nfmark target and match support: module (pass)
        CONFIG_NETFILTER_XT_SET: set target and match support: module (pass)
        CONFIG_NETFILTER_XT_TARGET_MASQUERADE: MASQUERADE target support: module (pass)
        CONFIG_NETFILTER_XT_NAT: "SNAT and DNAT" targets support: module (pass)
        CONFIG_NETFILTER_XT_MATCH_ADDRTYPE: "addrtype" address type match support: module (pass)
        CONFIG_NETFILTER_XT_MATCH_CONNTRACK: "conntrack" connection tracking match support: module (pass)
        CONFIG_NETFILTER_XT_MATCH_MULTIPORT: "multiport" Multiple port match support: module (pass)
        CONFIG_NETFILTER_XT_MATCH_RECENT: "recent" match support: module (pass)
        CONFIG_NETFILTER_XT_MATCH_STATISTIC: "statistic" match support: module (pass)
      CONFIG_NETFILTER_NETLINK: module (pass)
      CONFIG_NF_NAT: module (pass)
      CONFIG_IP_SET: IP set support: module (pass)
        CONFIG_IP_SET_HASH_IP: hash:ip set support: module (pass)
        CONFIG_IP_SET_HASH_NET: hash:net set support: module (pass)
      CONFIG_IP_VS: IP virtual server support: module (pass)
        CONFIG_IP_VS_NFCT: Netfilter connection tracking: built-in (pass)
        CONFIG_IP_VS_SH: Source hashing scheduling: module (pass)
        CONFIG_IP_VS_RR: Round-robin scheduling: module (pass)
        CONFIG_IP_VS_WRR: Weighted round-robin scheduling: module (pass)
      CONFIG_NF_CONNTRACK_IPV4: IPv4 connetion tracking support (required for NAT): unknown (warning)
      CONFIG_NF_REJECT_IPV4: IPv4 packet rejection: module (pass)
      CONFIG_NF_NAT_IPV4: IPv4 NAT: unknown (warning)
      CONFIG_IP_NF_IPTABLES: IP tables support: module (pass)
        CONFIG_IP_NF_FILTER: Packet filtering: module (pass)
          CONFIG_IP_NF_TARGET_REJECT: REJECT target support: module (pass)
        CONFIG_IP_NF_NAT: iptables NAT support: module (pass)
        CONFIG_IP_NF_MANGLE: Packet mangling: module (pass)
      CONFIG_NF_DEFRAG_IPV4: module (pass)
      CONFIG_NF_CONNTRACK_IPV6: IPv6 connetion tracking support (required for NAT): unknown (warning)
      CONFIG_NF_NAT_IPV6: IPv6 NAT: unknown (warning)
      CONFIG_IP6_NF_IPTABLES: IP6 tables support: module (pass)
        CONFIG_IP6_NF_FILTER: Packet filtering: module (pass)
        CONFIG_IP6_NF_MANGLE: Packet mangling: module (pass)
        CONFIG_IP6_NF_NAT: ip6tables NAT support: module (pass)
      CONFIG_NF_DEFRAG_IPV6: module (pass)
    CONFIG_BRIDGE: 802.1d Ethernet Bridging: module (pass)
      CONFIG_LLC: module (pass)
      CONFIG_STP: module (pass)
  CONFIG_EXT4_FS: The Extended 4 (ext4) filesystem: built-in (pass)
  CONFIG_PROC_FS: /proc file system support: built-in (pass)
```

## Deploy a node

Each node can now serve as a control plane node or worker node or both.

### As single node

This is a self-contained single node setup which runs both control plane
components and worker components. If you don't plan join any more nodes into the
cluster, this is for you.

Install the `k0scontroller` service:

```console
ubuntu@ubuntu:~$ sudo k0s install controller --single
ubuntu@ubuntu:~$ sudo systemctl status k0scontroller.service
○ k0scontroller.service - k0s - Zero Friction Kubernetes
     Loaded: loaded (/etc/systemd/system/k0scontroller.service; enabled; vendor preset: enabled)
     Active: inactive (dead)
       Docs: https://docs.k0sproject.io
```

Start it:

```console
ubuntu@ubuntu:~$ sudo systemctl start k0scontroller.service
ubuntu@ubuntu:~$ systemctl status k0scontroller.service
● k0scontroller.service - k0s - Zero Friction Kubernetes
     Loaded: loaded (/etc/systemd/system/k0scontroller.service; enabled; vendor preset: enabled)
     Active: active (running) since Thu 2022-08-18 09:56:02 UTC; 2s ago
       Docs: https://docs.k0sproject.io
   Main PID: 2720 (k0s)
      Tasks: 10
     Memory: 24.7M
        CPU: 4.654s
     CGroup: /system.slice/k0scontroller.service
             └─2720 /usr/local/bin/k0s controller --single=true

Aug 18 09:56:04 ubuntu k0s[2720]: 2022/08/18 09:56:04 [INFO] received CSR
Aug 18 09:56:04 ubuntu k0s[2720]: 2022/08/18 09:56:04 [INFO] generating key: rsa-2048
Aug 18 09:56:04 ubuntu k0s[2720]: 2022/08/18 09:56:04 [INFO] received CSR
Aug 18 09:56:04 ubuntu k0s[2720]: 2022/08/18 09:56:04 [INFO] generating key: rsa-2048
Aug 18 09:56:04 ubuntu k0s[2720]: 2022/08/18 09:56:04 [INFO] received CSR
Aug 18 09:56:04 ubuntu k0s[2720]: 2022/08/18 09:56:04 [INFO] generating key: rsa-2048
Aug 18 09:56:04 ubuntu k0s[2720]: 2022/08/18 09:56:04 [INFO] encoded CSR
Aug 18 09:56:04 ubuntu k0s[2720]: 2022/08/18 09:56:04 [INFO] signed certificate with serial number 6275509116227039894094374442676315636193163621
Aug 18 09:56:04 ubuntu k0s[2720]: 2022/08/18 09:56:04 [INFO] encoded CSR
Aug 18 09:56:04 ubuntu k0s[2720]: 2022/08/18 09:56:04 [INFO] signed certificate with serial number 336800507542010809697469355930007636411790073226
```

When the cluster is up, try to have a look:

```console
ubuntu@ubuntu:~$ sudo k0s kc get nodes -owide
NAME     STATUS   ROLES           AGE     VERSION       INTERNAL-IP    EXTERNAL-IP   OS-IMAGE             KERNEL-VERSION      CONTAINER-RUNTIME
ubuntu   Ready    control-plane   4m41s   v{{{ extra.k8s_version }}}+k0s   10.152.56.54   <none>        Ubuntu 22.04.1 LTS   5.15.0-1013-raspi   containerd://1.7.23
ubuntu@ubuntu:~$ sudo k0s kc get pod -owide -A
NAMESPACE     NAME                              READY   STATUS    RESTARTS   AGE     IP             NODE     NOMINATED NODE   READINESS GATES
kube-system   kube-proxy-kkv2l                  1/1     Running   0          4m44s   10.152.56.54   ubuntu   <none>           <none>
kube-system   kube-router-vf2pv                 1/1     Running   0          4m44s   10.152.56.54   ubuntu   <none>           <none>
kube-system   coredns-88b745646-wd4mp           1/1     Running   0          5m10s   10.244.0.2     ubuntu   <none>           <none>
kube-system   metrics-server-7d7c4887f4-ssk49   1/1     Running   0          5m6s    10.244.0.3     ubuntu   <none>           <none>
```

Overall, the single k0s node uses less than 1 GiB of RAM:

```console
ubuntu@ubuntu:~$ free -h
               total        used        free      shared  buff/cache   available
Mem:           3.7Gi       715Mi       1.3Gi       3.0Mi       1.7Gi       2.8Gi
Swap:             0B          0B          0B
```

### As a controller node

This will install k0s as a single non-HA controller. It won't be able to run any
workloads, so you need to connect more workers to it.

Install the `k0scontroller` service. Note that we're not specifying any flags:

```console
ubuntu@ubuntu:~$ sudo k0s install controller
ubuntu@ubuntu:~$ systemctl status k0scontroller.service
○ k0scontroller.service - k0s - Zero Friction Kubernetes
     Loaded: loaded (/etc/systemd/system/k0scontroller.service; enabled; vendor preset: enabled)
     Active: inactive (dead)
       Docs: https://docs.k0sproject.io
```

Start it:

```console
ubuntu@ubuntu:~$ sudo systemctl start k0scontroller.service
ubuntu@ubuntu:~$ systemctl status k0scontroller.service
● k0scontroller.service - k0s - Zero Friction Kubernetes
     Loaded: loaded (/etc/systemd/system/k0scontroller.service; enabled; vendor preset: enabled)
     Active: active (running) since Thu 2022-08-18 10:31:07 UTC; 3s ago
       Docs: https://docs.k0sproject.io
   Main PID: 1176 (k0s)
      Tasks: 10
     Memory: 30.2M
        CPU: 8.936s
     CGroup: /system.slice/k0scontroller.service
             └─1176 /usr/local/bin/k0s controller

Aug 18 10:31:09 ubuntu k0s[1176]: 2022/08/18 10:31:09 [INFO] signed certificate with serial number 723202396395786987172578079268287418983457689579
Aug 18 10:31:09 ubuntu k0s[1176]: 2022/08/18 10:31:09 [INFO] encoded CSR
Aug 18 10:31:09 ubuntu k0s[1176]: 2022/08/18 10:31:09 [INFO] signed certificate with serial number 36297085497443583023060005045470362249819432477
Aug 18 10:31:09 ubuntu k0s[1176]: 2022/08/18 10:31:09 [INFO] encoded CSR
Aug 18 10:31:09 ubuntu k0s[1176]: 2022/08/18 10:31:09 [INFO] encoded CSR
Aug 18 10:31:09 ubuntu k0s[1176]: 2022/08/18 10:31:09 [INFO] signed certificate with serial number 728910847354665355109188021924183608444435075827
Aug 18 10:31:09 ubuntu k0s[1176]: 2022/08/18 10:31:09 [INFO] generate received request
Aug 18 10:31:09 ubuntu k0s[1176]: 2022/08/18 10:31:09 [INFO] received CSR
Aug 18 10:31:09 ubuntu k0s[1176]: 2022/08/18 10:31:09 [INFO] generating key: rsa-2048
Aug 18 10:31:09 ubuntu k0s[1176]: 2022/08/18 10:31:09 [INFO] signed certificate with serial number 718948898553094584370065610752227487244528071083
```

As soon as the controller is up, we can try to inspect the API as we did for the
single node:

```console
ubuntu@ubuntu:~$ sudo k0s kc get nodes -owide
No resources found
ubuntu@ubuntu:~$ sudo k0s kc get pod -owide -A
NAMESPACE     NAME                              READY   STATUS    RESTARTS   AGE   IP       NODE     NOMINATED NODE   READINESS GATES
kube-system   coredns-88b745646-6tpwm           0/1     Pending   0          29s   <none>   <none>   <none>           <none>
kube-system   metrics-server-7d7c4887f4-9k5k5   0/1     Pending   0          24s   <none>   <none>   <none>           <none>
```

As we see, there are no nodes and two pending pods. A control plane without
workers. The memory consumption is below the single node controller, but not
much:

```console
ubuntu@ubuntu:~$ free -h
               total        used        free      shared  buff/cache   available
Mem:           3.7Gi       678Mi       2.3Gi       3.0Mi       758Mi       2.9Gi
Swap:             0B          0B          0B
```

This controller runs a full-fledged control plane, backed by [etcd], as opposed
to the lightweight [kine] based one from the single node example. For the
latter, k0s doesn't support joining new nodes.

More nodes can be added by creating join tokens. To add a worker node, create a
token for it:

```console
ubuntu@ubuntu:~$ sudo k0s token create --role worker
H4sIAAAAAAAC/2yV0Y6jPBKF7/MUeYGZ30DonUTai5+Ak5DgbhuXHXwHmAnBhtAJHdKs9t1XnZmRdqW9K1cdfceyrDqzvD+L6no7X7rV/O7MSvtxG6rrbTX7Nv9dr2bz+Xx+q6736rqa18PQ31Z//eWg747vfvdfvvuL1cti4T1VZXUdzj/PZT5U3/KPob5cz8PnN50P+Wp+SNFwSJ01Ax3zcxAyEUMKKqYIA3vO0LA2TpwCC1hEQipFrxD2UogDhawQobWJY297jxHBCdbS70hIvWKTOMWGBcwhgUaMSegPhdPH+VY13GDGYNxTiwONdMSEJtTiLeVYMMALDn6dOKqXtt5r0WfQPpqK43cpWKBAecnWktxEiAvWVZEDghPCorhmXTlWp/7PTPz3jEPcVZF6p0KsFfIlNZiIiB11iFUhlJ+1jkxwn/EjU4kRnnI1zsEJkkiH4OHt2pI4a0gEINZUYEEhQinEkUb4qU0Rvn+9CQD5UKJ0dKfG1NVZ2dWCcfCkHFDKycjbYZuGIsk5DngY7Svcn3N5mdIGm1yylkU+Srcxyiy7l50ZRUTvGqtcNuK9QAvEjcihu4yJh/sipC5xy4nBssut9UrcB6nENz72JnfxKLBmxAseZftgyhHvfLIjaeK+PNYX2tmwkKQrGjPlSFAI2VRKmyZmidjnsGCefRfe6Vp4p6veBk0FCtaN/uBu7JAp9kS6nFKDCQvxVUXYsGPiFji+VU05UtFvdLt8oVK8JRE+5m6fZfbvBcGa8QhH0pzG6vxjLEOSEJvtZdRvhNSywNmCejEihiRMYp/IH34utZc6GpdwWwgbc9Hhh5Q+4ushLeXJEZ6t85YBCLxTTfwmGhyWW+HC2B+AE1DnYdK4l9pYJ/P0jhn1mrsq1MbHKYqcRO6cyuAQQG/kRlsq2aOK/HVp2FZKDVRqQg0OmNuz3MTB2jgBiXSQCGHYVmN6XnoAItDIrmnbBxDFHbdqB8ZZU5ktGMRAgQUApzuH3chQ9BCSRcrBR2riVCHxBt5ln3kYlXKxKKI6JEizV4wn3tWyMMk1N/iVtvpayvqaQ+nrKfj6gxMzOOCIBF/+cBQv4JG4AnATe0GZjUNy6gcWkkG5CJGpntKGTnzb472XfeqtekuQzqsWua+bpaw2j9d0ih02YZauh5y4/v7gqZzY2lYmVuWkahFqzF0cri1jbPu3n4d6nVp10G4fVw3OZbp8VabfaQfvtWN9zYNOdfVYmIWjz4PMzOOFmv5Nb3u39CgqXdUCth4xyxrwaQ8Oc3On9xIet3mHmewCj7kJgmP/pr3os5i0oLx+1+4yyj1mcwuTmDIko50DpndhWwNxHwcQQSuEGFljI0Z7lYJ1EhgnguJ3PukPYXr3VbJYOCdE5ECSFpBqgrDEpzFzRSfFxSUgIrJhUQZxW5jazxpCk445CfK3RMbHdcOGtL2N0O7uAuyCId8A0izZ4B2EseQb55EgwVX7+CyjmB9c1eSTVQXeLWiDj4CjUW7ZXXl9nR7pqDYKUXnZqyZ4r46x98bR/vduxtzQE0UiFZHdpEACEcFzLx/o5Z+z+bzL22o1N+g2Ky/dUD2GXznxq/6VE39C46n6anzcnqePorLV8K24XIbbcM37/6V9XK9VN3z7Q3o2zbnTq/n60v08n2b9tfpZXauurG6r+b/+PfuiPs1/Q/4P/mn8vMJwMVW3mrvL84/lj+8N8ia/uZ/Lf2izWFb57D8BAAD//zANvmsEBwAA
```

Save the join token for subsequent steps.

[etcd]: https://etcd.io
[kine]: https://github.com/k3s-io/kine

### As a worker node

To join an existing k0s cluster, create the join token file for the worker
(where `$TOKEN_CONTENT` is one of the join tokens created in the control plane
setup):

```console
sudo sh -c 'mkdir -p /var/lib/k0s/ && umask 077 && echo "$TOKEN_CONTENT" > /var/lib/k0s/join-token'
```

After that, install the `k0sworker` service:

```console
ubuntu@ubuntu:~$ sudo k0s install worker --token-file /var/lib/k0s/join-token
ubuntu@ubuntu:~$ systemctl status k0sworker.service
○ k0sworker.service - k0s - Zero Friction Kubernetes
     Loaded: loaded (/etc/systemd/system/k0sworker.service; enabled; vendor preset: enabled)
     Active: inactive (dead)
       Docs: https://docs.k0sproject.io
```

Start the service:

```console
ubuntu@ubuntu:~$ sudo systemctl start k0sworker.service
ubuntu@ubuntu:~$ systemctl status k0sworker.service
● k0sworker.service - k0s - Zero Friction Kubernetes
     Loaded: loaded (/etc/systemd/system/k0sworker.service; enabled; vendor preset: enabled)
     Active: active (running) since Thu 2022-08-18 13:48:58 UTC; 2s ago
       Docs: https://docs.k0sproject.io
   Main PID: 1631 (k0s)
      Tasks: 22
     Memory: 181.7M
        CPU: 4.010s
     CGroup: /system.slice/k0sworker.service
             ├─1631 /usr/local/bin/k0s worker --token-file=/var/lib/k0s/join-token
             └─1643 /var/lib/k0s/bin/containerd --root=/var/lib/k0s/containerd --state=/run/k0s/containerd --address=/run/k0s/containerd.sock --log-level=info --config=/etc/k0s/containerd.toml

Aug 18 13:49:00 ubuntu k0s[1631]: time="2022-08-18 13:49:00" level=info msg="Starting to supervise" component=containerd
Aug 18 13:49:00 ubuntu k0s[1631]: time="2022-08-18 13:49:00" level=info msg="Started successfully, go nuts pid 1643" component=containerd
Aug 18 13:49:00 ubuntu k0s[1631]: time="2022-08-18 13:49:00" level=info msg="starting OCIBundleReconciler"
Aug 18 13:49:00 ubuntu k0s[1631]: time="2022-08-18 13:49:00" level=info msg="starting Kubelet"
Aug 18 13:49:00 ubuntu k0s[1631]: time="2022-08-18 13:49:00" level=info msg="Starting kubelet"
Aug 18 13:49:00 ubuntu k0s[1631]: time="2022-08-18 13:49:00" level=info msg="detected 127.0.0.53 nameserver, assuming systemd-resolved, so using resolv.conf: /run/systemd/resolve/resolv.conf"
Aug 18 13:49:00 ubuntu k0s[1631]: time="2022-08-18 13:49:00" level=info msg="Starting to supervise" component=kubelet
Aug 18 13:49:00 ubuntu k0s[1631]: time="2022-08-18 13:49:00" level=info msg="Started successfully, go nuts pid 1648" component=kubelet
Aug 18 13:49:00 ubuntu k0s[1631]: time="2022-08-18 13:49:00" level=info msg="starting Status"
Aug 18 13:49:00 ubuntu k0s[1631]: time="2022-08-18 13:49:00" level=info msg="starting Autopilot"
```

As this is a worker node, we cannot access the Kubernetes API via the builtin
`k0s kc` subcommand, but we can check the k0s API instead:

```console
ubuntu@ubuntu:~$ sudo k0s status
Version: v{{{ extra.k8s_version }}}+k0s.0
Process ID: 1631
Role: worker
Workloads: true
SingleNode: false
```

The memory requirements are also pretty low:

```console
ubuntu@ubuntu:~$ free -h
               total        used        free      shared  buff/cache   available
Mem:           3.7Gi       336Mi       2.1Gi       3.0Mi       1.2Gi       3.2Gi
Swap:             0B          0B          0B
```

## Connect to the cluster

On a controller node, generate a new `raspi-cluster-master` user with admin
rights and get a [kubeconfig] for it:

```console
ubuntu@ubuntu:~$ sudo k0s kc create clusterrolebinding raspi-cluster-master-admin --clusterrole=admin --user=raspi-cluster-master
clusterrolebinding.rbac.authorization.k8s.io/raspi-cluster-master-admin created
ubuntu@ubuntu:~$ sudo k0s kubeconfig create --groups system:masters raspi-cluster-master

apiVersion: v1
clusters:
- cluster:
    server: https://10.152.56.54:6443
    certificate-authority-data: LS0tLS1CRUdJTiBDRVJUSUZJQ0FURS0tLS0tCk1JSURBRENDQWVpZ0F3SUJBZ0lVT2RSVzdWdm83UWR5dmdFZHRUK1V3WDN2YXdvd0RRWUpLb1pJaHZjTkFRRUwKQlFBd0dERVdNQlFHQTFVRUF4TU5hM1ZpWlhKdVpYUmxjeTFqWVRBZUZ3MHlNakE0TVRneE5EQTFNREJhRncwegpNakE0TVRVeE5EQTFNREJhTUJneEZqQVVCZ05WQkFNVERXdDFZbVZ5Ym1WMFpYTXRZMkV3Z2dFaU1BMEdDU3FHClNJYjNEUUVCQVFVQUE0SUJEd0F3Z2dFS0FvSUJBUURsdy8wRFJtcG1xRjVnVElmN1o5bElRN0RFdUp6WDJLN1MKcWNvYk5oallFanBqbnBDaXFYOSt5T1R2cGgyUlRKN2tvaGkvUGxrYm5oM2pkeVQ3NWxSMGowSkV1elRMaUdJcApoR2pqc3htek5RRWVwb210R0JwZXNGeUE3NmxTNVp6WVJtT0lFQVgwb0liWjBZazhuU3pQaXBsWDMwcTFETEhGCkVIcSsyZG9vVXRIb09EaEdmWFRJTUJsclZCV3dCV3cxbmdnN0dKb01TN2tHblpYaUw2NFBiRDg5NmtjYXo0a28KTXhhZGc1ZmZQNStBV3JIVHhKV1d2YjNCMjEyOWx3R3FiOHhMTCt1cnVISHVjNEh4em9OVUt1WUlXc2lvQWp4YgphdDh6M1QwV2RnSit2VithWWlRNFlLeEVFdFB4cEMvUHk0czU0UHF4RzVZa0hiMDczMEUxQWdNQkFBR2pRakJBCk1BNEdBMVVkRHdFQi93UUVBd0lCQmpBUEJnTlZIUk1CQWY4RUJUQURBUUgvTUIwR0ExVWREZ1FXQkJTd2p4STIKRUxVNCtNZUtwT0JNQUNnZDdKU1QxVEFOQmdrcWhraUc5dzBCQVFzRkFBT0NBUUVBQ3k3dHFFMk5WT3E0Z0I1Ngp2clVZMFU0SWp1c0dUN0UzQ2xqSUtQODk2Mm9xdlpvU0NWb2U5YS9UQTR6ZXYrSXJwaTZ1QXFxc3RmT3JFcDJ4CmVwMWdYZHQrbG5nV0xlbXdWdEVOZ0xvSnBTM09Vc3N1ai9XcmJwSVU4M04xWVJTRzdzU21KdXhpa3pnVUhiUk8KZ01SLzIxSDFESzJFdmdQY2pHWXlGbUQzSXQzSjVNcnNiUHZTRG4rUzdWWWF0eWhIMUo4dmwxVDFpbzRWWjRTNgpJRFlaV05JOU10TUpqcGxXL01pRnlwTUhFU1E3UEhHeHpGVExoWFplS0pKSlRPYXFha1AxM3J1WFByVHVDQkl4CkFCSWQraU9qdGhSU3ZxbTFocGtHcmY4Rm9PdG1PYXZmazdDdnNJTWdUV2pqd2JJZWZIRU8zUmVBMzZWZWV3bXoKOFJHVUtBPT0KLS0tLS1FTkQgQ0VSVElGSUNBVEUtLS0tLQo=
  name: k0s
contexts:
- context:
    cluster: k0s
    user: raspi-cluster-master
  name: k0s
current-context: k0s
kind: Config
preferences: {}
users:
- name: raspi-cluster-master
  user:
    client-certificate-data: LS0tLS1CRUdJTiBDRVJUSUZJQ0FURS0tLS0tCk1JSURYVENDQWtXZ0F3SUJBZ0lVV0ZZNkZ4cCtUYnhxQUxTVjM0REVMb0dEc3Q0d0RRWUpLb1pJaHZjTkFRRUwKQlFBd0dERVdNQlFHQTFVRUF4TU5hM1ZpWlhKdVpYUmxjeTFqWVRBZUZ3MHlNakE0TVRneE5ERTRNREJhRncweQpNekE0TVRneE5ERTRNREJhTURneEZ6QVZCZ05WQkFvVERuTjVjM1JsYlRwdFlYTjBaWEp6TVIwd0d3WURWUVFECkV4UnlZWE53YVMxamJIVnpkR1Z5TFcxaGMzUmxjakNDQVNJd0RRWUpLb1pJaHZjTkFRRUJCUUFEZ2dFUEFEQ0MKQVFvQ2dnRUJBTGJNalI5eHA1dDJzank1S0dEQnQ2dWl3QU4vaEhwZkFUNXJrZTFRblc2eFlZeDYzR2JBTXYrRQpjWmEyUEdPempQeVVTZThVdWp4ZnR0L1JWSTJRVkVIRGlJZ1ZDNk1tUUFmTm1WVlpKOHBFaTM2dGJZYUVxN3dxClhxYmJBQ0F0ZGtwNTJ0Y0RLVU9sRS9SV0tUSjN4bXUvRmh0OTIrRDdtM1RrZTE0TkJ5a1hvakk1a2xVWU9ySEMKVTN3V210eXlIUFpDMFBPdWpXSE5yeS9wOXFjZzRreWNDN0NzUVZqMWoxY2JwdXRpWllvRHNHV3piS0RTbExRZApyYnUwRnRVZVpUQzVPN2NuTk5tMU1EZldubXhlekw4L2N5dkJCYnRmMjhmcERFeEhMT2dTY2ZZUlZwUllPMzdvCk5yUjljMGNaZE9oZW5YVnlQcU1WVVlSNkQxMlRrY0VDQXdFQUFhTi9NSDB3RGdZRFZSMFBBUUgvQkFRREFnV2cKTUIwR0ExVWRKUVFXTUJRR0NDc0dBUVVGQndNQkJnZ3JCZ0VGQlFjREFqQU1CZ05WSFJNQkFmOEVBakFBTUIwRwpBMVVkRGdRV0JCUitqQTlGNm1jc25ob2NtMnd0dFNYY2tCaUpoakFmQmdOVkhTTUVHREFXZ0JTd2p4STJFTFU0CitNZUtwT0JNQUNnZDdKU1QxVEFOQmdrcWhraUc5dzBCQVFzRkFBT0NBUUVBY2RRV3N4OUpHOUIxckxVc2Y1QzgKd1BzTkhkZURYeG1idm4zbXN3aFdVMEZHU1pjWjlkMTYzeXhEWnA4QlNzNWFjNnZqcU1lWlFyRThDUXdXYTlxVAowZVJXcTlFODYzcS9VcFVNN3lPM1BnMHd4RWtQSTVuSjRkM0o3MHA3Zk4zenpzMUJzU0h6Q2hzOWR4dE5XaVp5CnNINzdhbG9NanA0cXBEVWRyVWcyT0d4RWhRdzJIaXE3ZEprQm80a3hoWmhBc3lWTDdZRng0SDY3WkIzSjY4V3QKdTdiWnRmUVJZV3ZPUE9oS0pFdmlLVXptNDJBUlZXTDdhZHVESTBBNmpxbXhkTGNxKzlNWVlaNm1CT0NWakx1WgoybDlJSVI2NkdjOUdpdC9kSFdwbTVZbmozeW8xcUU0UVg4ZmVUQTczUlU5cmFIdkNpTGdVbFRaVUNGa3JNL0NtCndBPT0KLS0tLS1FTkQgQ0VSVElGSUNBVEUtLS0tLQo=
    client-key-data: LS0tLS1CRUdJTiBSU0EgUFJJVkFURSBLRVktLS0tLQpNSUlFb3dJQkFBS0NBUUVBdHN5TkgzR25tM2F5UExrb1lNRzNxNkxBQTMrRWVsOEJQbXVSN1ZDZGJyRmhqSHJjClpzQXkvNFJ4bHJZOFk3T00vSlJKN3hTNlBGKzIzOUZValpCVVFjT0lpQlVMb3laQUI4MlpWVmtueWtTTGZxMXQKaG9TcnZDcGVwdHNBSUMxMlNubmExd01wUTZVVDlGWXBNbmZHYTc4V0czM2I0UHViZE9SN1hnMEhLUmVpTWptUwpWUmc2c2NKVGZCYWEzTEljOWtMUTg2Nk5ZYzJ2TCtuMnB5RGlUSndMc0t4QldQV1BWeHVtNjJKbGlnT3daYk5zCm9OS1V0QjJ0dTdRVzFSNWxNTGs3dHljMDJiVXdOOWFlYkY3TXZ6OXpLOEVGdTEvYngra01URWNzNkJKeDloRlcKbEZnN2Z1ZzJ0SDF6UnhsMDZGNmRkWEkrb3hWUmhIb1BYWk9Sd1FJREFRQUJBb0lCQUFpYytzbFFnYVZCb29SWgo5UjBhQTUyQ3ZhbHNpTUY3V0lPb2JlZlF0SnBTb1ZZTk0vVmplUU94S2VrQURUaGxiVzg1VFlLR1o0QVF3bjBwClQrS2J1bHllNmYvL2ZkemlJSUk5bmN2M3QzaEFZcEpGZWJPczdLcWhGSFNvUFFsSEd4dkhRaGgvZmFKQ1ZQNWUKVVBLZjBpbWhoMWtrUlFnRTB2NWZCYkVZekEyVGl4bThJSGtQUkdmZWN4WmF1VHpBS2VLR0hjTFpDem8xRHhlSgp3bHpEUW9YWDdHQnY5MGxqR1pndENXcFEyRUxaZ1NwdW0rZ0crekg1WFNXZXgwMzJ4d0NhbkdDdGcyRmxHd2V2Ck9PaG8zSjNrRWVJR1MzSzFJY24rcU9hMjRGZmgvcmRsWXFSdStWeEZ4ZkZqWGxaUjdjZkF4Mnc1Z3NmWm9CRXIKUE1oMTdVRUNnWUVBejZiTDc4RWsvZU1jczF6aWdaVVpZcE5qa2FuWHlsS3NUUWM1dU1pRmNORFdObFkxdlQzVQprOHE5cHVLbnBZRVlTTGVVTS9tSWk5TVp6bmZjSmJSL0hJSG9YVjFMQVJ2blQ0djN3T0JsaDc5ajdKUjBpOW1OClYrR0Q1SlNPUmZCVmYxVlJHRXN6d3ZhOVJsS2lMZ0JVM2tKeWN2Q09jYm5aeFltSXRrbDhDbXNDZ1lFQTRWeG4KZTY2QURIYmR3T0plbEFSKytkVHh5eVYyRjY1SEZDNldPQVh2RVRucGRudnRRUUprWWhNYzM1Y2gvMldmZDBWYQpZb3lGZE9kRThKZSsvcWxuS1pBc3BHRC9yZHp2VmFteHQ4WXdrQXU5Q1diZWw2VENPYkZOQ2hjK1NUbmRqN0duCmlSUHprM1JYMnBEVi9OaW5FVFA0TEJnTHJQYkxlSVAwSzZ4bjk0TUNnWUVBeXZGMmNVendUVjRRNTgrSTVDS0gKVzhzMnpkOFRzbjVZUFRRcG1zb0hlTG55RWNyeDNKRTRXSFVXSTZ0ek01TFczQUxuU21DL3JnQlVRWER0Yk1CYQpWczh6L1VPM2tVN25JOXhrK0ZHWGlUTnBnb2VZM0RGMExZYVBNL0JvbUR3S0kxZUwyVlZ1TWthWnQ4ZjlEejV0CnM0ZDNlWlJYY3hpem1KY1JVUzdDbHg4Q2dZQk45Vmc2K2RlRCtFNm4zZWNYenlKWnJHZGtmZllISlJ1amlLWWcKaFRUNVFZNVlsWEF5Yi9CbjJQTEJDaGdSc0lia2pKSkN5eGVUcERrOS9WQnQ2ZzRzMjVvRjF5UTdjZFU5VGZHVApnRFRtYjVrYU9vSy85SmZYdTFUS0s5WTVJSkpibGZvOXVqQWxqemFnL2o5NE16NC8vamxZajR6aWJaRmZoRTRnCkdZanhud0tCZ0U1cFIwMlVCa1hYL3IvdjRqck52enNDSjR5V3U2aWtpem00UmJKUXJVdEVNd1Y3a2JjNEs0VFIKM2s1blo1M1J4OGhjYTlMbXREcDJIRWo2MlBpL2pMR0JTN0NhOCtQcStxNjZwWWFZTDAwWnc4UGI3OVMrUmpzQQpONkNuQWg1dDFYeDhVMTIvWm9JcjBpOWZDaERuNlBqVEM0MVh5M1EwWWd6TW5jYXMyNVBiCi0tLS0tRU5EIFJTQSBQUklWQVRFIEtFWS0tLS0tCg==
```

Using the above kubeconfig, you can now access and use the cluster:

```console
ubuntu@ubuntu:~$ KUBECONFIG=/path/to/kubeconfig kubectl get nodes,deployments,pods -owide -A
NAME          STATUS   ROLES    AGE    VERSION       INTERNAL-IP    EXTERNAL-IP   OS-IMAGE             KERNEL-VERSION      CONTAINER-RUNTIME
node/ubuntu   Ready    <none>   5m1s   v{{{ extra.k8s_version }}}+k0s   10.152.56.54   <none>        Ubuntu 22.04.1 LTS   5.15.0-1013-raspi   containerd://1.7.23

NAMESPACE     NAME                             READY   UP-TO-DATE   AVAILABLE   AGE   CONTAINERS       IMAGES                                                 SELECTOR
kube-system   deployment.apps/coredns          1/1     1            1           33m   coredns          registry.k8s.io/coredns/coredns:v1.7.0                 k8s-app=kube-dns
kube-system   deployment.apps/metrics-server   1/1     1            1           33m   metrics-server   registry.k8s.io/metrics-server/metrics-server:v0.7.2   k8s-app=metrics-server

NAMESPACE     NAME                                  READY   STATUS    RESTARTS   AGE    IP             NODE     NOMINATED NODE   READINESS GATES
kube-system   pod/coredns-88b745646-pkk5w           1/1     Running   0          33m    10.244.0.5     ubuntu   <none>           <none>
kube-system   pod/konnectivity-agent-h4nfj          1/1     Running   0          5m1s   10.244.0.6     ubuntu   <none>           <none>
kube-system   pod/kube-proxy-qcgzs                  1/1     Running   0          5m1s   10.152.56.54   ubuntu   <none>           <none>
kube-system   pod/kube-router-6lrht                 1/1     Running   0          5m1s   10.152.56.54   ubuntu   <none>           <none>
kube-system   pod/metrics-server-7d7c4887f4-wwbkk   1/1     Running   0          33m    10.244.0.4     ubuntu   <none>           <none>
```

[kubeconfig]: https://kubernetes.io/docs/concepts/configuration/organize-cluster-access-kubeconfig/
