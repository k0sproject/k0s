# Networking

## In-cluster networking

k0s supports any standard [CNI] network providers. For convenience, k0s does come bundled with tho built-in providers, [Kube-router] and [Calico].

[CNI]: https://github.com/containernetworking/cni
[Kube-router]: https://github.com/cloudnativelabs/kube-router
[Calico]: https://www.projectcalico.org/

### Custom CNI configuration

You can opt-out of having k0s manage the network setup and choose instead to use any network plugin that adheres to the CNI specification. To do so, configure `custom` as the [network provider] in the k0s configuration file (`k0s.yaml`). To deploy the CNI provider you want to use, you can deploy it using Helm, plain Kubernetes manifests or any other way.

[network provider]: configuration.md#specnetwork

### Notes

- When deploying k0s with the default settings, all pods on a node can communicate with all pods on all nodes. No configuration changes are needed to get started.
- Once you initialize the cluster with a network provider the only way to change providers is through a full cluster redeployment.

### Kube-router

Kube-router is built into k0s, and so by default the distribution uses it for network provision. Kube-router uses the standard Linux networking stack and toolset, and you can set up CNI networking without any overlays by using BGP as the main mechanism for in-cluster networking.

- Uses bit less resources (~15%)
- Does NOT support Windows nodes

### Calico

In addition to kube-router, k0s also offers [Calico] as an alternative,
integrated network provider. Calico is a layer 3 container networking solution
that routes packets to pods. For example, it supports pod-specific network
policies that help secure Kubernetes clusters in demanding use cases. Calico in
k0s uses VXLAN by default. However, IP in IP is also supported via the `bird`
mode.

- Uses bit more resources
- Supports Windows nodes

## Controller-Worker communication

One goal of k0s is to allow for the deployment of an isolated control plane, which may prevent the establishment of an IP route between controller nodes and the pod network. Thus, to enable this communication path (which is mandated by conformance tests), k0s deploys [Konnectivity service](https://kubernetes.io/docs/tasks/extend-kubernetes/setup-konnectivity/) to proxy traffic from the API server (control plane) into the worker nodes. This ensures that we can always fulfill all the Kubernetes API functionalities, but still operate the control plane in total isolation from the workers.

**Note**: To allow Konnectivity agents running on the worker nodes to establish the connection, configure your firewalls for outbound access, port 8132. Moreover, configure your firewalls for outbound access, port 6443, in order to access Kube-API from the worker nodes.

![k0s controller_worker_networking](img/k0s_controller_worker_networking.png)

## Required ports and protocols

| Protocol | Port  | Service        | Direction                      | Notes
|----------|-------|----------------|--------------------------------|--------
| TCP      | 2380  | etcd peers     | controller <-> controller      |
| TCP      | 6443  | kube-apiserver | worker, CLI => controller      | Authenticated Kube API using Kube TLS client certs, ServiceAccount tokens with RBAC
| TCP      | 179   | kube-router    | worker <-> worker              | BGP routing sessions between peers
| UDP      | 4789  | Calico         | worker <-> worker              | Calico VXLAN overlay
| TCP      | 10250 | kubelet        | controller, worker => host `*` | Authenticated kubelet API for the controller node `kube-apiserver` (and `heapster`/`metrics-server` addons) using TLS client certs
| TCP      | 9443  | k0s-api        | controller <-> controller      | k0s controller join API, TLS with token auth
| TCP      | 8132  | konnectivity   | worker <-> controller          | Konnectivity is used as "reverse" tunnel between kube-apiserver and worker kubelets
| TCP      | 112   | keepalived     | controller <-> controller      | Only required for control plane load balancing vrrpInstances for ip address 224.0.0.18. 224.0.0.18 is a multicast IP address defined in [RFC 3768].

You also need enable all traffic to and from the [podCIDR and serviceCIDR] subnets on nodes with a worker role.

[podCIDR and serviceCIDR]: configuration.md#specnetwork
[RFC 3768]: https://datatracker.ietf.org/doc/html/rfc3768#section-5.2.2

## iptables

`iptables` can work in two distinct modes, `legacy` and `nftables`. k0s autodetects the mode and prefers `nftables`. To check which mode k0s is configured with check `ls -lah /var/lib/k0s/bin/`. The `iptables` link target reveals the mode which k0s selected. k0s has the same logic as other k8s components, but to ensure al component have picked up the same mode you can check via:
**kube-proxy**: `nsenter -t $(pidof kube-proxy) -m iptables -V`
**kube-router**: `nsenter -t $(pidof kube-router) -m /sbin/iptables -V`
**calico**: `nsenter -t $(pidof -s calico-node) -m iptables -V`

There are [known](https://bugzilla.netfilter.org/show_bug.cgi?id=1632) version incompatibility issues in iptables versions. k0s ships (in `/var/lib/k0s/bin`) a version of iptables that is tested to interoperate with all other Kubernetes components it ships with. However if you have other tooling (firewalls etc.) on your hosts that uses iptables and the host iptables version is different that k0s (and other k8s components) ships with it may cause networking issues. This is based on the fact that iptables being user-space tooling it does not provide any strong version compatibility guarantees.

## Firewalld & k0s

If you are using [`firewalld`](https://firewalld.org/) on your hosts you need to ensure it is configured to use the same `FirewallBackend` as k0s and other Kubernetes components use. Otherwise networking will be broken in various ways.

Here's an example configuration for a tested working networking setup:

```sh
[root@rhel-test ~]# firewall-cmd --list-all
public (active)
  target: default
  icmp-block-inversion: no
  interfaces: eth0
  sources: 10.244.0.0/16 10.96.0.0/12
  services: cockpit dhcpv6-client ssh
  ports: 80/tcp 6443/tcp 8132/tcp 10250/tcp 179/tcp 179/udp
  protocols: 
  forward: no
  masquerade: yes
  forward-ports: 
  source-ports: 
  icmp-blocks: 
  rich rules:
```

Basic single node firewalld setup:

```sh
# Allow traffic from podCIDR
$ firewall-cmd --add-source=10.244.0.0/16 --permanent
# Allow kubelet API traffic
$ firewall-cmd --add-port=10250/tcp --permanent
# Apply firewall changes
$ firewall-cmd --reload
```
