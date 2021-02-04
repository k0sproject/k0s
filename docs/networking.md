# Networking

## In-cluster networking

k0s uses Calico as the default, built-in network provider. Calico is a container networking solution making use of layer 3 to route packets to pods. It supports for example pod specific network policies helping to secure kubernetes clusters in demanding use cases. Calico uses vxlan overlay network by default. Also ipip (IP-in-IP) is supported by configuration.

When deploying k0s with the default settings, all pods on a node can communicate with all pods on all nodes. No configuration changes are needed to get started.

It is possible for a user to opt-out of Calico and k0s managing the network. Users are able to utilize any network plugin following the CNI specification. By configuring `custom` as the network provider (in k0s.yaml) it is expected that the user sets up the networking. This can be achieved e.g. by pushing network provider manifests into `/var/lib/k0s/manifests` from where k0s controllers will pick them up and deploy into the cluster. More on the automatic manifest handling [here](manifests.md).

## Controller(s) - Worker communication

As one of the goals of k0s is to allow deployment of totally isolated control plane we cannot rely on the fact that there is an IP route between controller nodes and the pod overlay network. To enable this communication path, which is mandated by conformance tests, we use [Konnectivity service](https://kubernetes.io/docs/tasks/extend-kubernetes/setup-konnectivity/) to proxy the traffic from API server (control plane) into the worker nodes. Possible firewalls should be configured with outbound access so that Konnectivity agents running on the worker nodes can establish the connection. This ensures that we can always fulfill all the Kubernetes API functionalities, but still operate the control plane in total isolation from the workers.

![k0s controller_worker_networking](img/k0s_controller_worker_networking.png)

## Needed open ports & protocols

| Protocol  |  Port     | Service                   | Direction                   | Notes  
|-----------|-----------|---------------------------|-----------------------------|--------
| TCP       | 2380      | etcd peers                | controller <-> controller   |   
| TCP       | 6443      | kube-apiserver            | Worker, CLI => controller   | authenticated kube API using kube TLS client certs, ServiceAccount tokens with RBAC
| UDP       | 4789      | Calico                    | worker <-> worker           | Calico VXLAN overlay 
| TCP       | 10250     | kubelet                   | Master, Worker => Host `*`  | authenticated kubelet API for the master node `kube-apiserver` (and `heapster`/`metrics-server` addons) using TLS client certs 
| TCP       | 9443      | k0s-api                   | controller <-> controller   | k0s controller join API, TLS with token auth
| TCP       | 8132,8133 | konnectivity server       | worker <-> controller       | konnectivity is used as "reverse" tunnel between kube-apiserver and worker kubelets

