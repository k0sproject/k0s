# Networking

## In-cluster networking

k0s supports two Container Network Interface (CNI) providers out-of-box, [Kube-router](https://github.com/cloudnativelabs/kube-router) and [Calico](https://www.projectcalico.org/). In addition, k0s can support your own CNI configuration.

### Notes

- When deploying k0s with the default settings, all pods on a node can communicate with all pods on all nodes. No configuration changes are needed to get started.
- Once you initialize the cluster with a network provider the only way to change providers is through a full cluster redeployment.

### Kube-router

Kube-router is built into k0s, and so by default the distribution uses it for network provision. Kube-router uses the standard Linux networking stack and toolset, and you can set up CNI networking without any overlays by using BGP as the main mechanism for in-cluster networking.

- Supports armv7 (among many other archs)
- Uses bit less resources (~15%)
- Does NOT support dual-stack (IPv4/IPv6) networking
- Does NOT support Windows nodes
- Does NOT activate hairpin mode by default

### Calico

In addition to Kube-router, k0s also offers [Calico](https://www.projectcalico.org/) as an alternative, built-in network provider. Calico is a layer 3 container networking solution that routes packets to pods. It supports, for example, pod-specific network policies that help to secure kubernetes clusters in demanding use cases. Calico uses the vxlan overlay network by default, and you can configure it to support ipip (IP-in-IP).

- Does NOT support armv7
- Uses bit more resources
- Supports dual-stack (IPv4/IPv6) networking
- Supports Windows nodes

### Custom CNI configuration

You can opt-out of having k0s manage the network setup and choose instead to use any network plugin that adheres to the CNI specification. To do so, configure `custom` as the network provider in the k0s configurtion file (`k0s.yaml`). You can do this, for example, by pushing network provider manifests into `/var/lib/k0s/manifests`, from where k0s controllers will collect them for deployment into the cluster (for more information, refer to [Manifest Deployer](manifests.md).

## Controller-Worker communication

One goal of k0s is to allow for the deployment of an isolated control plane, which may prevent the establishment of an IP route between controller nodes and the pod network. Thus, to enable this communication path (which is mandated by conformance tests), k0s deploys [Konnectivity service](https://kubernetes.io/docs/tasks/extend-kubernetes/setup-konnectivity/) to proxy traffic from the API server (control plane) into the worker nodes. This ensures that we can always fulfill all the Kubernetes API functionalities, but still operate the control plane in total isolation from the workers.

**Note**: To allow Konnectivity agents running on the worker nodes to establish the connection, configure your firewalls for outbound access, port 8132. Moreover, configure your firewalls for outbound access, port 6443, in order to access Kube-API from the worker nodes.

![k0s controller_worker_networking](img/k0s_controller_worker_networking.png)

## Required ports and protocols

| Protocol  |  Port     | Service                   | Direction                   | Notes
|-----------|-----------|---------------------------|-----------------------------|--------
| TCP       | 2380      | etcd peers                | controller <-> controller   |
| TCP       | 6443      | kube-apiserver            | Worker, CLI => controller   | Authenticated Kube API using Kube TLS client certs, ServiceAccount tokens with RBAC
| TCP       | 179       | kube-router               | worker <-> worker           | BGP routing sessions between peers
| UDP       | 4789      | Calico                    | worker <-> worker           | Calico VXLAN overlay
| TCP       | 10250     | kubelet                   | Master, Worker => Host `*`  | Authenticated kubelet API for the master node `kube-apiserver` (and `heapster`/`metrics-server` addons) using TLS client certs
| TCP       | 9443      | k0s-api                   | controller <-> controller   | k0s controller join API, TLS with token auth
| TCP       | 8132      | konnectivity              | worker <-> controller       | Konnectivity is used as "reverse" tunnel between kube-apiserver and worker kubelets
