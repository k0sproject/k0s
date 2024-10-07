# Control plane load balancing

For clusters that don't have an [externally managed load balancer](high-availability.md#load-balancer) for the k0s
control plane, there is another option to get a highly available control plane called control plane load balancing (CPLB).

CPLB has two features that are independent, but normally will be used together: VRRP Instances, which allows
automatic assignation of predefined IP addresses using VRRP across control plane nodes. VirtualServers allows to
do Load Balancing to the other control plane nodes.

This feature is intended to be used for external traffic. This feature is fully compatible with
[node-local load balancing (NLLB)](nllb.md) which means CPLB can be used for external traffic and NLLB for
internal traffic at the same time.

## Technical functionality

The k0s control plane load balancer provides k0s with virtual IPs and TCP
load Balancing on each controller node. This allows the control plane to
be highly available using VRRP (Virtual Router Redundancy Protocol) and
IPVS long as the network infrastructure allows multicast and GARP.

[Keepalived](https://www.keepalived.org/) is the only load balancer that is
supported so far. Currently there are no plans to support other alternatives.

## VRRP Instances

VRRP, or Virtual Router Redundancy Protocol, is a protocol that allows several
routers to utilize the same virtual IP address. A VRRP instance refers to a
specific configuration of this protocol.

Each VRRP instance must have a unique virtualRouterID, at least one IP address,
one unique password (which is sent in plain text across your network, this is
to prevent accidental conflicts between VRRP instances) and one network
interface.

Except for the network interface, all the fields of a VRRP instance must have
the same value on all the control plane nodes.

Usually, users will define multiple VRRP instances when they need k0s to be
highly available on multiple network interfaces.

## Enabling in a cluster

In order to use control plane load balancing, the cluster needs to comply with the
following:

* K0s isn't running as a [single node](k0s-single-node.md), i.e. it isn't
  started using the `--single` flag.
* The cluster should have multiple controller nodes. Technically CPLB also works
  with a single controller node, but is only useful in conjunction with a highly
  available control plane.
* Unique virtualRouterID and authPass for each VRRP Instance in the same broadcast domain.
  These do not provide any sort of security against ill-intentioned attacks, they are
  safety features to prevent accidental conflicts between VRRP instances in the same
  network segment.
* If `VirtualServers` are used, the cluster configuration mustn't specify a non-empty
  [`spec.api.externalAddress`][specapi]. If only `VRRPInstances` are specified, a
  non-empty [`spec.api.externalAddress`][specapi] may be specified.

Add the following to the cluster configuration (`k0s.yaml`):

```yaml
spec:
  network:
    controlPlaneLoadBalancing:
      enabled: true
      type: Keepalived
      keepalived:
        vrrpInstances:
        - virtualIPs: ["<External address IP>/<external address IP netmask"]
          authPass: <password>
        virtualServers:
        - ipAddress: "ipAddress"
```

Or alternatively, if using [`k0sctl`](k0sctl-install.md), add the following to
the k0sctl configuration (`k0sctl.yaml`):

```yaml
spec:
  k0s:
    config:
      spec:
        network:
          controlPlaneLoadBalancing:
            enabled: true
            type: Keepalived
            keepalived:
              vrrpInstances:
              - virtualIPs: ["<External address IP>/<external address IP netmask>"]
                authPass: <password>
              virtualServers:
              - ipAddress: "<External ip address>"
```

Because this is a feature intended to configure the apiserver, CPLB does not
support dynamic configuration and in order to make changes you need to restart
the k0s controllers to make changes.

[specapi]: configuration.md#specapi

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
    version: v{{{ extra.k8s_version }}}+k0s.0
    config:
      spec:
        api:
          sans:
          - 192.168.122.200
        network:
          controlPlaneLoadBalancing:
            enabled: true
            type: Keepalived
            keepalived:
              vrrpInstances:
              - virtualIPs: ["192.168.122.200/24"]
                authPass: Example
              virtualServers:
              - ipAddress: "<External ip address>"
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
k0sctl  Copyright 2023, k0sctl authors.
Anonymized telemetry of usage will be sent to the authors.
By continuing to use k0sctl you agree to these terms:
https://k0sproject.io/licenses/eula
level=info msg="==> Running phase: Connect to hosts"
level=info msg="[ssh] worker-2.k0s.lab:22: connected"
level=info msg="[ssh] controller-2.k0s.lab:22: connected"
level=info msg="[ssh] worker-1.k0s.lab:22: connected"
level=info msg="[ssh] worker-0.k0s.lab:22: connected"
level=info msg="[ssh] controller-0.k0s.lab:22: connected"
level=info msg="[ssh] controller-1.k0s.lab:22: connected"
level=info msg="==> Running phase: Detect host operating systems"
level=info msg="[ssh] worker-2.k0s.lab:22: is running Fedora Linux 38 (Cloud Edition)"
level=info msg="[ssh] controller-2.k0s.lab:22: is running Fedora Linux 38 (Cloud Edition)"
level=info msg="[ssh] controller-0.k0s.lab:22: is running Fedora Linux 38 (Cloud Edition)"
level=info msg="[ssh] controller-1.k0s.lab:22: is running Fedora Linux 38 (Cloud Edition)"
level=info msg="[ssh] worker-0.k0s.lab:22: is running Fedora Linux 38 (Cloud Edition)"
level=info msg="[ssh] worker-1.k0s.lab:22: is running Fedora Linux 38 (Cloud Edition)"
level=info msg="==> Running phase: Acquire exclusive host lock"
level=info msg="==> Running phase: Prepare hosts"
level=info msg="==> Running phase: Gather host facts"
level=info msg="[ssh] worker-2.k0s.lab:22: using worker-2.k0s.lab as hostname"
level=info msg="[ssh] controller-0.k0s.lab:22: using controller-0.k0s.lab as hostname"
level=info msg="[ssh] controller-2.k0s.lab:22: using controller-2.k0s.lab as hostname"
level=info msg="[ssh] controller-1.k0s.lab:22: using controller-1.k0s.lab as hostname"
level=info msg="[ssh] worker-1.k0s.lab:22: using worker-1.k0s.lab as hostname"
level=info msg="[ssh] worker-0.k0s.lab:22: using worker-0.k0s.lab as hostname"
level=info msg="[ssh] worker-2.k0s.lab:22: discovered eth0 as private interface"
level=info msg="[ssh] controller-0.k0s.lab:22: discovered eth0 as private interface"
level=info msg="[ssh] controller-2.k0s.lab:22: discovered eth0 as private interface"
level=info msg="[ssh] controller-1.k0s.lab:22: discovered eth0 as private interface"
level=info msg="[ssh] worker-1.k0s.lab:22: discovered eth0 as private interface"
level=info msg="[ssh] worker-0.k0s.lab:22: discovered eth0 as private interface"
level=info msg="[ssh] worker-2.k0s.lab:22: discovered 192.168.122.210 as private address"
level=info msg="[ssh] controller-0.k0s.lab:22: discovered 192.168.122.37 as private address"
level=info msg="[ssh] controller-2.k0s.lab:22: discovered 192.168.122.87 as private address"
level=info msg="[ssh] controller-1.k0s.lab:22: discovered 192.168.122.185 as private address"
level=info msg="[ssh] worker-1.k0s.lab:22: discovered 192.168.122.81 as private address"
level=info msg="[ssh] worker-0.k0s.lab:22: discovered 192.168.122.219 as private address"
level=info msg="==> Running phase: Validate hosts"
level=info msg="==> Running phase: Validate facts"
level=info msg="==> Running phase: Download k0s binaries to local host"
level=info msg="==> Running phase: Upload k0s binaries to hosts"
level=info msg="[ssh] controller-0.k0s.lab:22: uploading k0s binary from /opt/k0s"
level=info msg="[ssh] controller-2.k0s.lab:22: uploading k0s binary from /opt/k0s"
level=info msg="[ssh] worker-0.k0s.lab:22: uploading k0s binary from /opt/k0s"
level=info msg="[ssh] controller-1.k0s.lab:22: uploading k0s binary from /opt/k0s"
level=info msg="[ssh] worker-1.k0s.lab:22: uploading k0s binary from /opt/k0s"
level=info msg="[ssh] worker-2.k0s.lab:22: uploading k0s binary from /opt/k0s"
level=info msg="==> Running phase: Install k0s binaries on hosts"
level=info msg="[ssh] controller-0.k0s.lab:22: validating configuration"
level=info msg="[ssh] controller-1.k0s.lab:22: validating configuration"
level=info msg="[ssh] controller-2.k0s.lab:22: validating configuration"
level=info msg="==> Running phase: Configure k0s"
level=info msg="[ssh] controller-0.k0s.lab:22: installing new configuration"
level=info msg="[ssh] controller-2.k0s.lab:22: installing new configuration"
level=info msg="[ssh] controller-1.k0s.lab:22: installing new configuration"
level=info msg="==> Running phase: Initialize the k0s cluster"
level=info msg="[ssh] controller-0.k0s.lab:22: installing k0s controller"
level=info msg="[ssh] controller-0.k0s.lab:22: waiting for the k0s service to start"
level=info msg="[ssh] controller-0.k0s.lab:22: waiting for kubernetes api to respond"
level=info msg="==> Running phase: Install controllers"
level=info msg="[ssh] controller-2.k0s.lab:22: validating api connection to https://192.168.122.200:6443"
level=info msg="[ssh] controller-1.k0s.lab:22: validating api connection to https://192.168.122.200:6443"
level=info msg="[ssh] controller-0.k0s.lab:22: generating token"
level=info msg="[ssh] controller-1.k0s.lab:22: writing join token"
level=info msg="[ssh] controller-1.k0s.lab:22: installing k0s controller"
level=info msg="[ssh] controller-1.k0s.lab:22: starting service"
level=info msg="[ssh] controller-1.k0s.lab:22: waiting for the k0s service to start"
level=info msg="[ssh] controller-1.k0s.lab:22: waiting for kubernetes api to respond"
level=info msg="[ssh] controller-0.k0s.lab:22: generating token"
level=info msg="[ssh] controller-2.k0s.lab:22: writing join token"
level=info msg="[ssh] controller-2.k0s.lab:22: installing k0s controller"
level=info msg="[ssh] controller-2.k0s.lab:22: starting service"
level=info msg="[ssh] controller-2.k0s.lab:22: waiting for the k0s service to start"
level=info msg="[ssh] controller-2.k0s.lab:22: waiting for kubernetes api to respond"
level=info msg="==> Running phase: Install workers"
level=info msg="[ssh] worker-2.k0s.lab:22: validating api connection to https://192.168.122.200:6443"
level=info msg="[ssh] worker-1.k0s.lab:22: validating api connection to https://192.168.122.200:6443"
level=info msg="[ssh] worker-0.k0s.lab:22: validating api connection to https://192.168.122.200:6443"
level=info msg="[ssh] controller-0.k0s.lab:22: generating a join token for worker 1"
level=info msg="[ssh] controller-0.k0s.lab:22: generating a join token for worker 2"
level=info msg="[ssh] controller-0.k0s.lab:22: generating a join token for worker 3"
level=info msg="[ssh] worker-2.k0s.lab:22: writing join token"
level=info msg="[ssh] worker-0.k0s.lab:22: writing join token"
level=info msg="[ssh] worker-1.k0s.lab:22: writing join token"
level=info msg="[ssh] worker-2.k0s.lab:22: installing k0s worker"
level=info msg="[ssh] worker-1.k0s.lab:22: installing k0s worker"
level=info msg="[ssh] worker-0.k0s.lab:22: installing k0s worker"
level=info msg="[ssh] worker-2.k0s.lab:22: starting service"
level=info msg="[ssh] worker-1.k0s.lab:22: starting service"
level=info msg="[ssh] worker-0.k0s.lab:22: starting service"
level=info msg="[ssh] worker-2.k0s.lab:22: waiting for node to become ready"
level=info msg="[ssh] worker-0.k0s.lab:22: waiting for node to become ready"
level=info msg="[ssh] worker-1.k0s.lab:22: waiting for node to become ready"
level=info msg="==> Running phase: Release exclusive host lock"
level=info msg="==> Running phase: Disconnect from hosts"
level=info msg="==> Finished in 2m20s"
level=info msg="k0s cluster version v{{{ extra.k8s_version }}}+k0s.0  is now installed"
level=info msg="Tip: To access the cluster you can now fetch the admin kubeconfig using:"
level=info msg="     k0sctl kubeconfig"
```

The cluster with the two nodes should be available by now. Setup the kubeconfig
file in order to interact with it:

```shell
k0sctl kubeconfig > k0s-kubeconfig
export KUBECONFIG=$(pwd)/k0s-kubeconfig
```

All three worker nodes are ready:

```console
$ kubectl get nodes
NAME                   STATUS   ROLES           AGE     VERSION
worker-0.k0s.lab       Ready    <none>          8m51s   v{{{ extra.k8s_version }}}+k0s
worker-1.k0s.lab       Ready    <none>          8m51s   v{{{ extra.k8s_version }}}+k0s
worker-2.k0s.lab       Ready    <none>          8m51s   v{{{ extra.k8s_version }}}+k0s
```

Each controller node has a dummy interface with the VIP and /32 netmask,
but only one has it in the real nic:

```console
$ for i in controller-{0..2} ; do echo $i ; ssh $i -- ip -4 --oneline addr show | grep -e eth0 -e dummyvip0; done
controller-0
2: eth0    inet 192.168.122.37/24 brd 192.168.122.255 scope global dynamic noprefixroute eth0\       valid_lft 2381sec preferred_lft 2381sec
2: eth0    inet 192.168.122.200/24 scope global secondary eth0\       valid_lft forever preferred_lft forever
3: dummyvip0    inet 192.168.122.200/32 scope global dummyvip0\       valid_lft forever preferred_lft forever
controller-1
2: eth0    inet 192.168.122.185/24 brd 192.168.122.255 scope global dynamic noprefixroute eth0\       valid_lft 2390sec preferred_lft 2390sec
3: dummyvip0    inet 192.168.122.200/32 scope global dummyvip0\       valid_lft forever preferred_lft forever
controller-2
2: eth0    inet 192.168.122.87/24 brd 192.168.122.255 scope global dynamic noprefixroute eth0\       valid_lft 2399sec preferred_lft 2399sec
3: dummyvip0    inet 192.168.122.200/32 scope global dummyvip0\       valid_lft forever preferred_lft forever
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
$ for i in controller-{0..2} ; do echo $i ; ssh $i -- ip -4 --oneline addr show | grep -e eth0 -e dummyvip0; done
controller-1
2: eth0    inet 192.168.122.185/24 brd 192.168.122.255 scope global dynamic noprefixroute eth0\       valid_lft 2173sec preferred_lft 2173sec
2: eth0    inet 192.168.122.200/24 scope global secondary eth0\       valid_lft forever preferred_lft forever
3: dummyvip0    inet 192.168.122.200/32 scope global dummyvip0\       valid_lft forever preferred_lft forever
controller-2
2: eth0    inet 192.168.122.87/24 brd 192.168.122.255 scope global dynamic noprefixroute eth0\       valid_lft 2182sec preferred_lft 2182sec
3: dummyvip0    inet 192.168.122.200/32 scope global dummyvip0\       valid_lft forever preferred_lft forever

$ for i in controller-{0..2} ; do echo $i ; ipvsadm --save -n; done
IP Virtual Server version 1.2.1 (size=4096)
Prot LocalAddress:Port Scheduler Flags
  -> RemoteAddress:Port           Forward Weight ActiveConn InActConn
TCP  192.168.122.200:6443 rr persistent 360
  -> 192.168.122.185:6443              Route   1      0          0
  -> 192.168.122.87:6443               Route   1      0          0
  -> 192.168.122.122:6443              Route   1      0          0
````

And the cluster will be working normally:

```console
$ kubectl get nodes
NAME                   STATUS   ROLES           AGE     VERSION
worker-0.k0s.lab       Ready    <none>          8m51s   v{{{ extra.k8s_version }}}+k0s
worker-1.k0s.lab       Ready    <none>          8m51s   v{{{ extra.k8s_version }}}+k0s
worker-2.k0s.lab       Ready    <none>          8m51s   v{{{ extra.k8s_version }}}+k0s
```
