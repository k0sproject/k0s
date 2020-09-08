# MKE Networking

## In-cluster networking

MKE supports currently only Calico as the built-in in-cluster overlay network provider. A user can however opt-out of mke managing the network setup by using a `custom` as the network type.

Using `custom` network provider it is expected that the user sets up the networking. This can be achieved e.g. by pushing network provider manifests into `/var/lib/mke/manifests` from where mke controllers will pick them up and deploy into the cluster. More on the automatic manifest handling [here](manifests.md).

## Controller(s) - Worker communication

As one of the goals of MKE is to allow deployment of totally isolated control plane we cannot rely on the fact that there is an IP route between controller nodes and the pod overlay network. To enable this communication path, which is mandated by conformance tests, we use [Egress service](https://kubernetes.io/docs/tasks/extend-kubernetes/setup-konnectivity/) and [konnectivity proxy](https://github.com/kubernetes-sigs/apiserver-network-proxy) to proxy the traffic from API server into worker nodes. This ansures that we can always fulfill all the Kubernetes API functionalities but still operate the control plane in total isolation from the workers.


## Needed open ports & protocols

| Protocol  |  Port     | Service                   | Direction                   | Notes  
|-----------|-----------|---------------------------|-----------------------------|--------
| TCP       | 2380      | etcd peers                | controller <-> controller   |   
| TCP       | 6443      | kube-apiserver            | Worker, CLI => controller   | authenticated kube API using kube TLS client certs, ServiceAccount tokens with RBAC
| UDP       | 4789      | Calico                    | worker <-> worker           | Calico VXLAN overlay 
| TCP       | 10250     | kubelet                   | Master, Worker => Host `*`  | authenticated kubelet API for the master node `kube-apiserver` (and `heapster`/`metrics-server` addons) using TLS client certs 
| TCP       | 9443      | mke-api                   | controller <-> controller   | MKE controller join API, TLS with token auth
| TCP       | 8132,8133 | konnectivity server       | worker <-> controller       | konnectivity is used as "reverse" tunnel between kube-apiserver and worker kubelets

