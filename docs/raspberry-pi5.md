<!--
SPDX-FileCopyrightText: 2025 k0s authors
SPDX-License-Identifier: CC-BY-SA-4.0
-->

# Create a Raspberry Pi 5 cluster

## Prerequisites

This guide assumes that you have a [Raspberry Pi 5] and a
sufficiently large SD card of at least 32 GB. We will be using [Raspberry Pi OS]
for this guide, though k0s should run just fine on other 64-bit Linux
distributions for the Raspberry Pi 5 as well. Please [file a Bug] if you encounter
any issues.

[Raspberry Pi 5]: https://www.raspberrypi.com/products/raspberry-pi-5/
[Raspberry Pi OS]: https://www.raspberrypi.com/software/
[file a Bug]: https://github.com/k0sproject/k0s/issues/new?assignees=&labels=bug&template=BUG_REPORT.yml

## Set up the system

Follow the official Raspberry Pi instructions on [booting Pi OS from the USB drive](https://projects.raspberrypi.org/en/projects/install-raspberry-pi-desktop/3) or another method depending on your needs.

### SSH Access

1. Typically, you can enable SSH while creating the bootable drive for Raspberry Pi OS. If not, you can enable it later

2. Add your public key to the server.
    From your host machine, run [ssh-copy-id] to copy your public SSH key to your Pi 5:

    ```bash
    ssh-copy-id -i ~/.ssh/id_rsa.pub <YOUR_USER_NAME>@<IP_ADDRESS_OF_THE_SERVER>
    ```

    When prompted, enter the password for your user account for the Pi. Your public key should be copied at the appropriate folder on the remote Pi automatically.

    Note: `~/.ssh/id_rsa.pub` is the default location for the public ssh key. If your key is elsewhere, adjust accordingly.

3. Verify SSH access

    ```shell
    ssh <YOUR_USER_NAME>@<IP_ADDRESS_OF_THE_SERVER>
    ```

    If your key has a paraphrase, you’ll be prompted for it.

[ssh-copy-id]: https://www.cyberciti.biz/faq/use-ssh-copy-id-with-an-openssh-server-listing-on-a-different-port/

### Enable the memory cgroup controller

Raspberry Pi OS does not enable the memory cgroup controller by default. However, it is required to run containerized workloads, so enable it:

1. Edit /boot/cmdline.txt:

    ```bash
    sudo nano /boot/cmdline.txt
    ```

    Append (on the same single line):

    ```bash
    cgroup_enable=memory cgroup_memory=1
    ```

2. Reboot

    ```bash
    sudo reboot
    ```

## Install k0s

### Download k0s

Download a [k0s release](https://github.com/k0sproject/k0s/releases/latest). For
example:

  ```shell
  wget -O /tmp/k0s https://github.com/k0sproject/k0s/releases/download/{{{ k0s_version }}}/k0s-{{{ k0s_version }}}-arm64 # replace version number!
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
  $ sudo k0s version
  {{{ k0s_version }}}
  ```

To check if k0s's [system requirements](system-requirements.md) and [external
runtime dependencies](external-runtime-deps.md) are fulfilled by your current
setup, you can invoke `k0s sysinfo`:

  ```console
  ramesses-pi5@pi:~ $ sudo k0s sysinfo
  Total memory: 7.9 GiB (pass)
  File system of /var/lib/k0s: ext4 (pass)
  Disk space available for /var/lib/k0s: 44.3 GiB (pass)
  Relative disk space available for /var/lib/k0s: 79% (pass)
  Name resolution: localhost: [::1 127.0.0.1] (pass)
  Operating system: Linux (pass)
    Linux kernel release: 6.6.51+rpt-rpi-2712 (pass)
    Max. file descriptors per process: current: 1048576 / max: 1048576 (pass)
    AppArmor: unavailable (pass)
    Executable in PATH: modprobe: /usr/sbin/modprobe (pass)
    Executable in PATH: mount: /usr/bin/mount (pass)
    Executable in PATH: umount: /usr/bin/umount (pass)
    /proc file system: mounted (0x9fa0) (pass)
    Control Groups: version 2 (pass)
      cgroup controller "cpu": available (is a listed root controller) (pass)
      cgroup controller "cpuacct": available (via cpu in version 2) (pass)
      cgroup controller "cpuset": available (is a listed root controller) (pass)
      cgroup controller "memory": available (is a listed root controller) (pass)
      cgroup controller "devices": available (device filters attachable) (pass)
      cgroup controller "freezer": available (cgroup.freeze exists) (pass)
      cgroup controller "pids": available (is a listed root controller) (pass)
      cgroup controller "hugetlb": unavailable (warning)
      cgroup controller "blkio": available (via io in version 2) (pass)
    CONFIG_CGROUPS: Control Group support: built-in (pass)
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
        CONFIG_IPV6: The IPv6 protocol: module (pass)
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
        CONFIG_NF_CONNTRACK_IPV4: IPv4 connection tracking support (required for NAT): unknown (warning)
        CONFIG_NF_REJECT_IPV4: IPv4 packet rejection: module (pass)
        CONFIG_NF_NAT_IPV4: IPv4 NAT: unknown (warning)
        CONFIG_IP_NF_IPTABLES: IP tables support: module (pass)
          CONFIG_IP_NF_FILTER: Packet filtering: module (pass)
            CONFIG_IP_NF_TARGET_REJECT: REJECT target support: module (pass)
          CONFIG_IP_NF_NAT: iptables NAT support: module (pass)
          CONFIG_IP_NF_MANGLE: Packet mangling: module (pass)
        CONFIG_NF_DEFRAG_IPV4: module (pass)
        CONFIG_NF_CONNTRACK_IPV6: IPv6 connection tracking support (required for NAT): unknown (warning)
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

### Deploy a Node using k0s

If you want a more hands-on process for setting up your Pi 5 devices as Kubernetes nodes with the k0s binary, refer to [the guide for Pi devices](./raspberry-pi4.md).

## Deploy a Node using k0sctl

### Install k0sctl on Your Host

Follow [the k0sctl installation guide](https://github.com/k0sproject/k0sctl#installation) and install k0sctl on your host machine.

### Single node K0s cluster

For this example, we'll create a cluster.yaml that describes your known Pi 5 device and use it as a single node (controller & worker) cluster, for example:

  ```yaml
  apiVersion: k0sctl.k0sproject.io/v1beta1
  kind: Cluster
  metadata:
    name: k0s-cluster
    user: admin
  spec:
    hosts:
    - ssh:
        address: <IP_ADDRESS_OF_THE_SERVER>
        user: <YOUR_USER_NAME>
        port: 22
        keyPath: ~/.ssh/id_rsa
      role: controller+worker
  ```

### SSH agent

  By default, k0sctl doesn’t prompt you for passphrases, so the easiest solution is to load your key into an SSH agent before running k0sctl. Here’s how you can do it:

  1. Start the SSH agent (if not already running)

      ```bash
      eval "$(ssh-agent -s)"
      ```

  2. Add your private key (you’ll be prompted for the passphrase)

      ```bash
      ssh-add ~/.ssh/id_rsa
      ```

  3. Verify the key is loaded

      ```bash
      ssh-add -l
      ```

### Deploy cluster

  1. Apply the cluster.yaml using k0sctl on your local machine.

      ```console
      $ k0sctl apply --config cluster.yaml

      ⠀⣿⣿⡇⠀⠀⢀⣴⣾⣿⠟⠁⢸⣿⣿⣿⣿⣿⣿⣿⡿⠛⠁⠀⢸⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⠀█████████ █████████ ███
      ⠀⣿⣿⡇⣠⣶⣿⡿⠋⠀⠀⠀⢸⣿⡇⠀⠀⠀⣠⠀⠀⢀⣠⡆⢸⣿⣿⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀███          ███    ███
      ⠀⣿⣿⣿⣿⣟⠋⠀⠀⠀⠀⠀⢸⣿⡇⠀⢰⣾⣿⠀⠀⣿⣿⡇⢸⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⠀███          ███    ███
      ⠀⣿⣿⡏⠻⣿⣷⣤⡀⠀⠀⠀⠸⠛⠁⠀⠸⠋⠁⠀⠀⣿⣿⡇⠈⠉⠉⠉⠉⠉⠉⠉⠉⢹⣿⣿⠀███          ███    ███
      ⠀⣿⣿⡇⠀⠀⠙⢿⣿⣦⣀⠀⠀⠀⣠⣶⣶⣶⣶⣶⣶⣿⣿⡇⢰⣶⣶⣶⣶⣶⣶⣶⣶⣾⣿⣿⠀█████████    ███    ██████████
      k0sctl v0.21.0 Copyright 2023, k0sctl authors.
      INFO ==> Running phase: Set k0s version
      INFO Looking up latest stable k0s version
      INFO Using k0s version {{{ k0s_version }}}
      INFO ==> Running phase: Connect to hosts
      INFO [ssh] 192.168.31.93:22: connected
      INFO ==> Running phase: Detect host operating systems
      INFO [ssh] 192.168.31.93:22: is running Debian GNU/Linux 12 (bookworm)
      INFO ==> Running phase: Acquire exclusive host lock
      INFO ==> Running phase: Prepare hosts
      INFO ==> Running phase: Gather host facts
      INFO [ssh] 192.168.31.93:22: using pi as hostname
      INFO [ssh] 192.168.31.93:22: discovered wlan0 as private interface
      INFO ==> Running phase: Validate hosts
      INFO ==> Running phase: Validate facts
      INFO ==> Running phase: Download k0s on hosts
      INFO [ssh] 192.168.31.93:22: downloading k0s {{{ k0s_version }}}
      INFO ==> Running phase: Install k0s binaries on hosts
      INFO [ssh] 192.168.31.93:22: validating configuration
      INFO ==> Running phase: Configure k0s
      INFO [ssh] 192.168.31.93:22: installing new configuration
      INFO ==> Running phase: Initialize the k0s cluster
      INFO [ssh] 192.168.31.93:22: installing k0s controller
      INFO [ssh] 192.168.31.93:22: waiting for the k0s service to start
      INFO [ssh] 192.168.31.93:22: wait for kubernetes to reach ready state
      INFO ==> Running phase: Release exclusive host lock
      INFO ==> Running phase: Disconnect from hosts
      INFO ==> Finished in 4m14s
      INFO k0s cluster version {{{ k0s_version }}} is now installed
      INFO Tip: To access the cluster you can now fetch the admin kubeconfig using:
      INFO      k0sctl kubeconfig --config cluster.yaml
      ```

  2. Fetch the kubeconfig use k0sctl.

      ```bash
        k0sctl kubeconfig --config cluster.yaml > pi_cluster.kubeconfig
      ```

  3. Export KUBECONFIG and verify

      ```console
      $ export KUBECONFIG=pi_cluster.kubeconfig
      $ kubectl get nodes
      NAME   STATUS   ROLES           AGE     VERSION
      pi     Ready    control-plane   2m54s   {{{ k8s_version }}}+k0s
      ```

## Tear down k0s on Pi 5

If you need to remove k0s entirely (for example, if you run into conflicts or just want a clean slate):

  1. Stop existing processes

      ```bash
      sudo k0s stop
      ```

  2. Reset k0s

      ```bash
      sudo k0s reset
      ```

  3. Remove k0s binaries

      ```bash
      sudo rm -rf /usr/local/bin/k0s
      ```

  4. Reboot Pi 5.

      ```bash
      sudo reboot
      ```
