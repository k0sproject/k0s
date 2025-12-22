<!--
SPDX-FileCopyrightText: 2024 k0s authors
SPDX-License-Identifier: CC-BY-SA-4.0
-->

# Control plane load balancing

For clusters that don't have an [externally managed load balancer](high-availability.md#load-balancer) for the k0s
control plane, there is another option to get a highly available control plane called control plane load balancing (CPLB).

CPLB provides clusters a highly available VIP (virtual IP) and load balancing for **accessing the cluster externally**.
For internal traffic (nodes to control plane) k0s provides [NLLB](nllb.md). Both features are fully compatible and it's
recommended to use both together if you don't have an external load balancer.

Load balancing means that an IP address will forward the traffic to every control plane node, Virtual IPs mean that
this IP address will be present on at least one node at a time.

CPLB relies on [keepalived](https://www.keepalived.org) for highly available VIPs. Internally, Keepalived uses the
[VRRP protocol](https://datatracker.ietf.org/doc/html/rfc3768). Load Balancing can be done through either userspace
reverse proxy implemented in k0s (recommended for simplicity), or it can use Keepalived's virtual servers feature,
which ultimately relies on IPVS.

## Compatibility

CPLB depends on multiple technologies to work together as a whole, making it difficult to work on every single scenario.

### Single node

CPLB is incompatible with running as a [single node](k0s-single-node.md). This means k0s must not be started using the `--single` flag.

### Controller + worker

K0s only supports the userspace reverse proxy load balancer. Keepalived's VirtualServers are not supported with controller + worker.

Both Kube-Router and Calico managed by k0s are supported with the userspace reverse proxy load balancer, however, k0s creates iptables
rules in the control plane nodes which may be incompatible with custom CNI plugins.

### External address

If [`spec.api.externalAddress`](configuration.md#specapi) is defined, control
plane load balancing implicitly
[disables](configuration.md#disabling-controller-components) k0s's endpoint
reconciler component, just as if the `--disable-components=endpoint-reconciler`
flag had been specified.

### Node Local load balancing

CPLB is fully compatible with [NLLB](nllb.md), however NLLB is incompatible with [`spec.api.externalAddress`](configuration.md#specapi).

## Virtual IPs - High availability

### What is a VIP (virtual IP)

A virtual IP is an IP address that isn't tied to a single network interface,
instead it floats between multiple servers. This is a failover mechanism that
grants that there is always at least a functioning server and removes a single
point of failure.

### Configuring VIPs

CPLB relies internally on Keepalived's VRRP Instances. A VRRP Instance is a
server that will manage one or more VIPs. Most users will need exactly one
VRRP instance with exactly one VIP, however k0s allows multiple VRRP servers
with multiple VIPs for more advanced use cases such as network segmentation.

A virtualIP requires:

1. A user-defined CIDR address which must be routable in the network. For most installations, this will be in the same CIDR as the physical interface.
**WARNING:** K0s is not aware of external IP address management and the administrator is responsible for ensuring that IP addresses aren't colliding.
2. A user-defined password which should be unique for each cluster. This password is a mechanism to prevent accidental conflicts. It's not encrypted
and doesn't prevent malicious attacks in any way.
3. A virtual router ID, which defaults to 51. This virtual router ID **must be unique** in the broadcast domain.
4. A network interface, if not defined, k0s will chose the network interface that owns the default route.

Except the network interface, all the other fields must be equal on every control plane node.

This is a minimal example:

```yaml
spec:
  network:
    controlPlaneLoadBalancing:
      enabled: true
      type: Keepalived
      keepalived:
        vrrpInstances:
        - virtualIPs: ["<VIP address>/<netmask>"] # for instance ["172.16.0.100/16"]
          authPass: "<my password>"
```

By default, VRRP Instances use multicast as per [RFC 3768]. It's possible to configure VRRP
instances to use unicast:

```yaml
spec:
  network:
    controlPlaneLoadBalancing:
      enabled: true
      type: Keepalived
      keepalived:
        vrrpInstances:
        - virtualIPs: ["<VIP address>/<netmask>"] # for instance ["172.16.0.100/16"]
          authPass: "<my password>"
          unicastSourceIP: <ip address of this controller>
          unicastPeers: [<ip address of other controllers>, ...]
```

When using unicast, k0st does not attempt to detect `unicastSourceIP` and it must be defined explicitly and
`unicastPeers` must include the IP address of the other controllers' `unicastSourceIP`.

[RFC 3768]: https://datatracker.ietf.org/doc/html/rfc3768#section-5.2.2

#### IPv6 VIPs egress routing preference

In IPv6 there aren't primary addresses, instead the routing preference is determined by the operating system using
either IP labels or IP rules. K0s Virtual Addresses uses [RFC 6724] IP labels. By default, k0s sets the label to
10000 so that it still uses the main IP address as a source of egress connections.

This value may be replaced per vrrPInstance:

```yaml
spec:
  network:
    controlPlaneLoadBalancing:
      enabled: true
      type: Keepalived
      keepalived:
        vrrpInstances:
        - virtualIPs: ["<VIP address>/<netmask>"] # for instance ["2001:db8:2::1/64"]
          authPass: "<my password>"
          addressLabel: 30000
```

The label value can be verified using iproute2:

```shell
$ ip addrlabel | grep 2001:db8:2::1
prefix 2001:db8:2::1/128 label 30000
```

The prefix always uses netmask 128.

K0s doesn't attempt to modify labels that do not belong to the VIP.

[RFC 6724]: https://datatracker.ietf.org/doc/html/rfc6724

## Load Balancing

Currently k0s allows to chose one of two load balancing mechanism:

1. A userspace reverse proxy running in the k0s process. This is the default and recommended setting.
2. For users who may need extra performance or more flexible algorithms, k0s can use the keepalived virtual servers load balancer feature.

All control plane nodes must use the same load balancing mechanism. Different Load balancing mechanism
is not supported and has undefined behavior.

### Load Balancing - Userspace Reverse Proxy

This is the default behavior, in order to enable it simple configure a VIP
using a VRRP instance.

```yaml
spec:
  network:
    controlPlaneLoadBalancing:
      enabled: true
      type: Keepalived
      keepalived:
        vrrpInstances:
        - virtualIPs: ["<VIP address>/<netmask>"] # for instance ["172.16.0.100/16"]
          authPass: "<my password>"
```

### Keepalived Virtual Servers Load Balancing

The Keepalived virtual servers Load Balancing is more performant than the userspace reverse proxy load balancer. However, it's
 not recommended because it has some drawbacks:

1. It's incompatible with controller+worker.
2. May not work on every infrastructure.
3. Troubleshooting is significantly more complex.
4. When there is more than one VRRPInstance, we must do the load balancing in all the servers
which in some rare circumstances can provoke temporary routing loops.

```yaml
spec:
  network:
    controlPlaneLoadBalancing:
      enabled: true
      type: Keepalived
      keepalived:
        vrrpInstances:
        - virtualIPs: ["<VIP address>/<netmask>"] # for instance ["172.16.0.100/16"]
          authPass: "<my password>"
        virtualServers:
        - ipAddress: "<VIP address without netmask>" # for instance 172.16.0.100
```

## Full example using `k0sctl`

The following example shows a full `k0sctl` configuration file featuring three
controllers and three workers with control plane load balancing enabled.

```yaml
apiVersion: k0sctl.k0sproject.io/v1beta1
kind: Cluster
metadata:
  name: k0s-cluster
spec:
  hosts:
  - role: controller
    ssh:
      address: controller-0.k0s.lab
      user: root
      keyPath: ~/.ssh/id_rsa
    k0sBinaryPath: /opt/k0s
    uploadBinary: true
  - role: controller
    ssh:
      address: controller-1.k0s.lab
      user: root
      keyPath: ~/.ssh/id_rsa
    k0sBinaryPath: /opt/k0s
    uploadBinary: true
  - role: controller
    ssh:
      address: controller-2.k0s.lab
      user: root
      keyPath: ~/.ssh/id_rsa
    k0sBinaryPath: /opt/k0s
    uploadBinary: true
  - role: worker
    ssh:
      address: worker-0.k0s.lab
      user: root
      keyPath: ~/.ssh/id_rsa
    k0sBinaryPath: /opt/k0s
    uploadBinary: true
  - role: worker
    ssh:
      address: worker-1.k0s.lab
      user: root
      keyPath: ~/.ssh/id_rsa
    k0sBinaryPath: /opt/k0s
    uploadBinary: true
  - role: worker
    ssh:
      address: worker-2.k0s.lab
      user: root
      keyPath: ~/.ssh/id_rsa
    k0sBinaryPath: /opt/k0s
    uploadBinary: true
  k0s:
    version: {{{ k0s_version }}}
    config:
      spec:
        network:
          controlPlaneLoadBalancing:
            enabled: true
            type: Keepalived
            keepalived:
              vrrpInstances:
              - virtualIPs: ["192.168.122.200/24"]
                authPass: Example
          nodeLocalLoadBalancing: # optional, but CPLB will often be used with NLLB.
            enabled: true
            type: EnvoyProxy
```

Save the above configuration into a file called `k0sctl.yaml` and apply it in
order to bootstrap the cluster:

```console
$ k0sctl apply
⠀⣿⣿⡇⠀⠀⢀⣴⣾⣿⠟⠁⢸⣿⣿⣿⣿⣿⣿⣿⡿⠛⠁⠀⢸⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⠀█████████ █████████ ███
⠀⣿⣿⡇⣠⣶⣿⡿⠋⠀⠀⠀⢸⣿⡇⠀⠀⠀⣠⠀⠀⢀⣠⡆⢸⣿⣿⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀███          ███    ███
⠀⣿⣿⣿⣿⣟⠋⠀⠀⠀⠀⠀⢸⣿⡇⠀⢰⣾⣿⠀⠀⣿⣿⡇⢸⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⠀███          ███    ███
⠀⣿⣿⡏⠻⣿⣷⣤⡀⠀⠀⠀⠸⠛⠁⠀⠸⠋⠁⠀⠀⣿⣿⡇⠈⠉⠉⠉⠉⠉⠉⠉⠉⢹⣿⣿⠀███          ███    ███
⠀⣿⣿⡇⠀⠀⠙⢿⣿⣦⣀⠀⠀⠀⣠⣶⣶⣶⣶⣶⣶⣿⣿⡇⢰⣶⣶⣶⣶⣶⣶⣶⣶⣾⣿⣿⠀█████████    ███    ██████████
k0sctl v0.21.0 Copyright 2023, k0sctl authors.
INFO ==> Running phase: Connect to hosts
INFO [ssh] worker-2.k0s.lab:22: connected
INFO [ssh] controller-2.k0s.lab:22: connected
INFO [ssh] worker-1.k0s.lab:22: connected
INFO [ssh] worker-0.k0s.lab:22: connected
INFO [ssh] controller-0.k0s.lab:22: connected
INFO [ssh] controller-1.k0s.lab:22: connected
INFO ==> Running phase: Detect host operating systems
INFO [ssh] worker-2.k0s.lab:22: is running Fedora Linux 38 (Cloud Edition)
INFO [ssh] controller-2.k0s.lab:22: is running Fedora Linux 38 (Cloud Edition)
INFO [ssh] controller-0.k0s.lab:22: is running Fedora Linux 38 (Cloud Edition)
INFO [ssh] controller-1.k0s.lab:22: is running Fedora Linux 38 (Cloud Edition)
INFO [ssh] worker-0.k0s.lab:22: is running Fedora Linux 38 (Cloud Edition)
INFO [ssh] worker-1.k0s.lab:22: is running Fedora Linux 38 (Cloud Edition)
INFO ==> Running phase: Acquire exclusive host lock
INFO ==> Running phase: Prepare hosts
INFO ==> Running phase: Gather host facts
INFO [ssh] worker-2.k0s.lab:22: using worker-2.k0s.lab as hostname
INFO [ssh] controller-0.k0s.lab:22: using controller-0.k0s.lab as hostname
INFO [ssh] controller-2.k0s.lab:22: using controller-2.k0s.lab as hostname
INFO [ssh] controller-1.k0s.lab:22: using controller-1.k0s.lab as hostname
INFO [ssh] worker-1.k0s.lab:22: using worker-1.k0s.lab as hostname
INFO [ssh] worker-0.k0s.lab:22: using worker-0.k0s.lab as hostname
INFO [ssh] worker-2.k0s.lab:22: discovered eth0 as private interface
INFO [ssh] controller-0.k0s.lab:22: discovered eth0 as private interface
INFO [ssh] controller-2.k0s.lab:22: discovered eth0 as private interface
INFO [ssh] controller-1.k0s.lab:22: discovered eth0 as private interface
INFO [ssh] worker-1.k0s.lab:22: discovered eth0 as private interface
INFO [ssh] worker-0.k0s.lab:22: discovered eth0 as private interface
INFO [ssh] worker-2.k0s.lab:22: discovered 192.168.122.210 as private address
INFO [ssh] controller-0.k0s.lab:22: discovered 192.168.122.37 as private address
INFO [ssh] controller-2.k0s.lab:22: discovered 192.168.122.87 as private address
INFO [ssh] controller-1.k0s.lab:22: discovered 192.168.122.185 as private address
INFO [ssh] worker-1.k0s.lab:22: discovered 192.168.122.81 as private address
INFO [ssh] worker-0.k0s.lab:22: discovered 192.168.122.219 as private address
INFO ==> Running phase: Validate hosts
INFO ==> Running phase: Validate facts
INFO ==> Running phase: Download k0s binaries to local host
INFO ==> Running phase: Upload k0s binaries to hosts
INFO [ssh] controller-0.k0s.lab:22: uploading k0s binary from /opt/k0s
INFO [ssh] controller-2.k0s.lab:22: uploading k0s binary from /opt/k0s
INFO [ssh] worker-0.k0s.lab:22: uploading k0s binary from /opt/k0s
INFO [ssh] controller-1.k0s.lab:22: uploading k0s binary from /opt/k0s
INFO [ssh] worker-1.k0s.lab:22: uploading k0s binary from /opt/k0s
INFO [ssh] worker-2.k0s.lab:22: uploading k0s binary from /opt/k0s
INFO ==> Running phase: Install k0s binaries on hosts
INFO [ssh] controller-0.k0s.lab:22: validating configuration
INFO [ssh] controller-1.k0s.lab:22: validating configuration
INFO [ssh] controller-2.k0s.lab:22: validating configuration
INFO ==> Running phase: Configure k0s
INFO [ssh] controller-0.k0s.lab:22: installing new configuration
INFO [ssh] controller-2.k0s.lab:22: installing new configuration
INFO [ssh] controller-1.k0s.lab:22: installing new configuration
INFO ==> Running phase: Initialize the k0s cluster
INFO [ssh] controller-0.k0s.lab:22: installing k0s controller
INFO [ssh] controller-0.k0s.lab:22: waiting for the k0s service to start
INFO [ssh] controller-0.k0s.lab:22: waiting for kubernetes api to respond
INFO ==> Running phase: Install controllers
INFO [ssh] controller-2.k0s.lab:22: validating api connection to https://192.168.122.200:6443
INFO [ssh] controller-1.k0s.lab:22: validating api connection to https://192.168.122.200:6443
INFO [ssh] controller-0.k0s.lab:22: generating token
INFO [ssh] controller-1.k0s.lab:22: writing join token
INFO [ssh] controller-1.k0s.lab:22: installing k0s controller
INFO [ssh] controller-1.k0s.lab:22: starting service
INFO [ssh] controller-1.k0s.lab:22: waiting for the k0s service to start
INFO [ssh] controller-1.k0s.lab:22: waiting for kubernetes api to respond
INFO [ssh] controller-0.k0s.lab:22: generating token
INFO [ssh] controller-2.k0s.lab:22: writing join token
INFO [ssh] controller-2.k0s.lab:22: installing k0s controller
INFO [ssh] controller-2.k0s.lab:22: starting service
INFO [ssh] controller-2.k0s.lab:22: waiting for the k0s service to start
INFO [ssh] controller-2.k0s.lab:22: waiting for kubernetes api to respond
INFO ==> Running phase: Install workers
INFO [ssh] worker-2.k0s.lab:22: validating api connection to https://192.168.122.200:6443
INFO [ssh] worker-1.k0s.lab:22: validating api connection to https://192.168.122.200:6443
INFO [ssh] worker-0.k0s.lab:22: validating api connection to https://192.168.122.200:6443
INFO [ssh] controller-0.k0s.lab:22: generating a join token for worker 1
INFO [ssh] controller-0.k0s.lab:22: generating a join token for worker 2
INFO [ssh] controller-0.k0s.lab:22: generating a join token for worker 3
INFO [ssh] worker-2.k0s.lab:22: writing join token
INFO [ssh] worker-0.k0s.lab:22: writing join token
INFO [ssh] worker-1.k0s.lab:22: writing join token
INFO [ssh] worker-2.k0s.lab:22: installing k0s worker
INFO [ssh] worker-1.k0s.lab:22: installing k0s worker
INFO [ssh] worker-0.k0s.lab:22: installing k0s worker
INFO [ssh] worker-2.k0s.lab:22: starting service
INFO [ssh] worker-1.k0s.lab:22: starting service
INFO [ssh] worker-0.k0s.lab:22: starting service
INFO [ssh] worker-2.k0s.lab:22: waiting for node to become ready
INFO [ssh] worker-0.k0s.lab:22: waiting for node to become ready
INFO [ssh] worker-1.k0s.lab:22: waiting for node to become ready
INFO ==> Running phase: Release exclusive host lock
INFO ==> Running phase: Disconnect from hosts
INFO ==> Finished in 2m20s
INFO k0s cluster version {{{ k0s_version }}} is now installed
INFO Tip: To access the cluster you can now fetch the admin kubeconfig using:
INFO      k0sctl kubeconfig
```

The cluster with the two nodes should be available by now. Setup the kubeconfig
file in order to interact with it:

```console
k0sctl kubeconfig > k0s-kubeconfig
export KUBECONFIG=$(pwd)/k0s-kubeconfig
```

All three worker nodes are ready:

```console
$ kubectl get nodes
NAME                   STATUS   ROLES           AGE     VERSION
worker-0.k0s.lab       Ready    <none>          8m51s   {{{ k8s_version }}}+k0s
worker-1.k0s.lab       Ready    <none>          8m51s   {{{ k8s_version }}}+k0s
worker-2.k0s.lab       Ready    <none>          8m51s   {{{ k8s_version }}}+k0s
```

Only one controller has the VIP:

```console
$ for i in controller-{0..2} ; do echo $i ; ssh $i -- ip -4 --oneline addr show | grep eth0; done
controller-0
2: eth0    inet 192.168.122.37/24 brd 192.168.122.255 scope global dynamic noprefixroute eth0\       valid_lft 2381sec preferred_lft 2381sec
2: eth0    inet 192.168.122.200/24 scope global secondary eth0\       valid_lft forever preferred_lft forever
controller-1
2: eth0    inet 192.168.122.185/24 brd 192.168.122.255 scope global dynamic noprefixroute eth0\       valid_lft 2390sec preferred_lft 2390sec
controller-2
2: eth0    inet 192.168.122.87/24 brd 192.168.122.255 scope global dynamic noprefixroute eth0\       valid_lft 2399sec preferred_lft 2399sec
```

The cluster is using control plane load balancing and is able to tolerate the
outage of one controller node. Shutdown the first controller to simulate a
failure condition:

```console
$ ssh controller-0 'sudo poweroff'
Connection to 192.168.122.37 closed by remote host.
```

Control plane load balancing provides high availability, the VIP will have moved to a different node:

```console
$ for i in controller-{1..2} ; do echo $i ; ssh $i -- ip -4 --oneline addr show | grep eth0; done
controller-1
2: eth0    inet 192.168.122.185/24 brd 192.168.122.255 scope global dynamic noprefixroute eth0\       valid_lft 2173sec preferred_lft 2173sec
2: eth0    inet 192.168.122.200/24 scope global secondary eth0\       valid_lft forever preferred_lft forever
controller-2
2: eth0    inet 192.168.122.87/24 brd 192.168.122.255 scope global dynamic noprefixroute eth0\       valid_lft 2182sec preferred_lft 2182sec
````

And the cluster will be working normally:

```console
$ kubectl get nodes
NAME                   STATUS   ROLES           AGE     VERSION
worker-0.k0s.lab       Ready    <none>          8m51s   {{{ k8s_version }}}+k0s
worker-1.k0s.lab       Ready    <none>          8m51s   {{{ k8s_version }}}+k0s
worker-2.k0s.lab       Ready    <none>          8m51s   {{{ k8s_version }}}+k0s
```

## Troubleshooting

Although Virtual IP addresses and load balancing work together and are closely
related, these are two independent processes and must be troubleshooting as two
independent features.

### Troubleshooting Virtual IPs

The first thing to check is that the VIP is present in exactly one node at a time,
for instance if a cluster has an `172.17.0.102/16` address and the interface is `eth0`,
the expected output is similar to:

```console
controller0:/# ip a s eth0
53: eth0@if54: <BROADCAST,MULTICAST,UP,LOWER_UP,M-DOWN> mtu 1500 qdisc noqueue state UP
    link/ether 02:42:ac:11:00:02 brd ff:ff:ff:ff:ff:ff
    inet 172.17.0.2/16 brd 172.17.255.255 scope global eth0
       valid_lft forever preferred_lft forever
    inet 172.17.0.102/16 scope global secondary eth0
       valid_lft forever preferred_lft forever
```

```console
controller1:/# ip a s eth0
55: eth0@if56: <BROADCAST,MULTICAST,UP,LOWER_UP,M-DOWN> mtu 1500 qdisc noqueue state UP
    link/ether 02:42:ac:11:00:03 brd ff:ff:ff:ff:ff:ff
    inet 172.17.0.3/16 brd 172.17.255.255 scope global eth0
       valid_lft forever preferred_lft forever
```

If the `virtualServers` feature is used, there must be a dummy interface on the
node called `dummyvip0` which has the VIP, but with a `/32` bits network mask.
This isn't the VIP and has to be there even if the VIP is held by another node.

```console
controller0:/# ip a s dummyvip0 | grep 172.17.0.102
    inet 172.17.0.102/32 scope global dummyvip0
```

```console
controller1:/# ip a s dummyvip0 | grep 172.17.0.102
    inet 172.17.0.102/32 scope global dummyvip0
```

If this isn't present in the nodes, keepalived logs can be seen in the k0s-logs, and
can be filtered with `component=keepalived`.

```console
controller0:/# journalctl -u k0scontroller | grep component=keepalived
time="2024-11-19 12:56:11" level=info msg="Starting to supervise" component=keepalived
time="2024-11-19 12:56:11" level=info msg="Started successfully, go nuts pid 409" component=keepalived
time="2024-11-19 12:56:11" level=info msg="Tue Nov 19 12:56:11 2024: Starting Keepalived v{{{ build_var('keepalived_version') }}}" component=keepalived stream=stderr
[...]
```

The Keepalived configuration is stored in a file called `keepalived.conf` in the
k0s run directory, by default `/run/k0s/keepalived.conf`, in this file there
should be a `vrrp_instance` section for each `vrrpInstance`.

Finally, k0s should have two keepalived processes running.

### Troubleshooting the Load Balancer's Endpoint List

Both the user space reverse proxy load balancer and Keepalived's virtual servers need an endpoint list to
do the load balancing. They share a component called `cplb-reconciler` which responsible for setting the
load balancer's endpoint list. This component monitors constantly the endpoint `kubernetes` in the
`default`namespace:

```console
controller0:/# kubectl get ep kubernetes -n default
NAME         ENDPOINTS                                         AGE
kubernetes   172.17.0.6:6443,172.17.0.7:6443,172.17.0.8:6443   9m14s
```

You can see the `cplb-reconciler` updates by running:

```console
controller0:/# journalctl -u k0scontroller | grep component=cplb-reconciler
time="2024-11-20 20:29:28" level=error msg="Failed to watch API server endpoints, last observed version is \"\", starting over in 10s ..." component=cplb-reconciler error="Get \"https://172.17.0.6:6443/api/v1/namespaces/default/endpoints?fieldSelector=metadata.name%3Dkubernetes&timeout=30s&timeoutSeconds=30\": dial tcp 172.17.0.6:6443: connect: connection refused"
time="2024-11-20 20:29:38" level=info msg="Updated the list of IPs: [172.17.0.6]" component=cplb-reconciler
time="2024-11-20 20:29:55" level=info msg="Updated the list of IPs: [172.17.0.6 172.17.0.7]" component=cplb-reconciler
time="2024-11-20 20:29:59" level=info msg="Updated the list of IPs: [172.17.0.6 172.17.0.7 172.17.0.8]" component=cplb-reconciler
```

### Troubleshooting the user space reverse proxy load balancer

The user space reverse proxy load balancer runs in the k0s process. It listens a separate socket, by default on port 6444:

```console
controller0:/# netstat -tlpn | grep 6444
tcp        0      0 :::6444                 :::*                    LISTEN      345/k0s
```

Then the requests to the VIP on the apiserver port are forwarded to this socket using one iptables rule:

```console
-A PREROUTING -d <VIP>/32 -p tcp -m tcp --dport <apiserver port> -j REDIRECT --to-ports <userspace proxy port>
```

A real life example of a cluster using the VIP `17.177.0.102` looks like:

```console
controller0:/# /var/lib/k0s/bin/iptables-save | grep 6444
-A PREROUTING -d 172.17.0.102/32 -p tcp -m tcp --dport 6443 -j REDIRECT --to-ports 6444
```

Keep in mind that clusters using IPv6 as a primary address family, should use ip6tables or ip6tables-save.

It the load balancer is not load balancing for whatever reason, you can establish connections to it directly. A good way to see if it's actually load balancing is checking the serving certificate:

```console
controller0:/# ip -o addr s eth0
43: eth0    inet 172.17.0.2/16 brd 172.17.255.255 scope global eth0\       valid_lft forever preferred_lft forever
43: eth0    inet 172.17.0.102/16 scope global secondary eth0\       valid_lft forever preferred_lft forever

controller0:/# openssl s_client -connect 172.17.0.102:6444  </dev/null 2>/dev/null | openssl x509 -noout -fingerprint
SHA1 Fingerprint=B7:90:E6:E4:E1:EE:5B:19:72:99:02:28:54:36:D9:84:D5:39:67:8B
controller0:/# openssl s_client -connect 172.17.0.102:6444  </dev/null 2>/dev/null | openssl x509 -noout -fingerprint
SHA1 Fingerprint=89:94:5C:E5:50:7E:40:B2:E5:20:E7:70:E8:58:91:ED:63:B0:EC:65
controller0:/# openssl s_client -connect 172.17.0.102:6444  </dev/null 2>/dev/null | openssl x509 -noout -fingerprint
SHA1 Fingerprint=49:0D:79:FD:79:6F:A0:E4:9D:BA:A1:65:9C:C5:54:CF:E5:20:BF:A8
controller0:/# openssl s_client -connect 172.17.0.102:6444  </dev/null 2>/dev/null | openssl x509 -noout -fingerprint
SHA1 Fingerprint=B7:90:E6:E4:E1:EE:5B:19:72:99:02:28:54:36:D9:84:D5:39:67:8B
```

Note: You can't query the port 6444 on the localhost address, there is an iptables conflict. You are expected to
be able to reach the port 6443 on any address and the port 6444 on any address except localhost.

### Troubleshooting Keepalived Virtual Servers

You can verify the Keepalived logs and configuration file using the steps described in the section
[troubleshooting virtual IPs](#troubleshooting-virtual-ips) above.

When virtual servers are enabled K0s generates two additional files:

* `keepalived-virtualservers-generated.conf`: This file contains the list of control plane nodes that should be balanced to.
* `keepalived-virtualservers-consumed.conf`: This is a symbolic link which points to `keepalived-virtualservers-generated.conf`
if the Keepalived VRRP instance's current state is `master` or to `/dev/null` if it's `backup`. This file is only generated if
there is exactly one VRRP instance.

Additionally, you can check the actual IPVS configuration using `ipvsadm`:

```console
controller0:/# ipvsadm --save -n
IP Virtual Server version 1.2.1 (size=4096)
Prot LocalAddress:Port Scheduler Flags
  -> RemoteAddress:Port           Forward Weight ActiveConn InActConn
TCP  192.168.122.200:6443 rr persistent 360
  -> 192.168.122.185:6443              Route   1      0          0
  -> 192.168.122.87:6443               Route   1      0          0
  -> 192.168.122.122:6443              Route   1      0          0
  ```

  In this example `192.168.122.200` is the virtual IP, and `192.168.122.185`, `192.168.122.87`
  and `192.168.122.122` are the control plane nodes.

If there is only one VRRP instance, only the current master should be load balancing.

## Custom Keepalived Templates

**Warning:** Any customization to Keepalived templates is outside the scope of k0s support.
Template variables are not considered a stable API and templates can break during upgrades. Use at your own risk.

K0s allows you to customize the Keepalived configuration by providing custom Go templates for advanced users
who need an extra layer of customization. A minimal example would be:

```yaml
spec:
  network:
    controlPlaneLoadBalancing:
      enabled: true
      type: Keepalived
      keepalived:
        vrrpInstances:
        - virtualIPs: ["<VIP address>/<netmask>"]
          authPass: "<my password>"
        configTemplateVRRP: /path/to/custom-vrrp-template.conf
        configTemplateVS: /path/to/custom-virtualservers-template.conf
```

It is recommended to create custom templates based on the default templates, these can be accessed using the following commands:

```shell
# View the default VRRP template
k0s keepalived-config vrrp

# View the default Virtual Servers template
k0s keepalived-config virtualservers
```

The templates are only read during the k0s bootstrap. If a template is changed it's required to restart k0s.

### Important Considerations

* **No Support**: Custom template configurations are not officially supported. If you encounter issues with custom templates, you may be asked to reproduce the issue with default templates.
* **Updates**: When upgrading k0s, review the default templates for any changes that may need to be incorporated into your custom templates.
* **Testing**: Always test custom templates thoroughly in a non-production environment before deploying to production.
* **Syntax Validation**: k0s performs minimal validation of custom templates. Invalid Keepalived configuration will cause the CPLB feature to fail.
