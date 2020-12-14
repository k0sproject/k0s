# Architecture

**Note:** As with any young project, things change rapidly. Thus all the details in this architecture documentation may not be always up-to-date, but the high level concepts and patterns should still apply.

## Packaging

k0s is packaged as single, self-extracting binary which embeds Kubernetes binaries. This has many benefits:
- Everything can be, and is, statically compiled
- No OS level deps
- No RPMs, dep's, snaps or any other OS specific packaging needed. Single "package" for all OSes
- We can fully control the versions of each and every dependency

![k0s packaging as a single binary](img/k0s_packaging.png)

## Control plane

k0s as a single binary acts as the process supervisor for all other control plane components. This means there's no container engine or kubelet running on controllers (by default). Which means there is no way for a cluster user to schedule workloads onto controller nodes.

![k0s Controller processes](img/k0s_controller_processes.png)

k0s creates, manages and configures each of the components. k0s runs all control plane components as "naked" processes. So on the controller node there's no container engine running.

### Storage

Typically Kubernetes control plane supports only etcd as the datastore. In addition to etcd, k0s supports many other datastore options. This is achieved by including [kine](https://github.com/rancher/kine/). Kine allows wide variety of backend data stores to be used such as MySQL, PostgreSQL, SQLite and dqlite. See more in storage [documentation](configuration.md#spec.storage)

In case of k0s managed etcd, k0s manages the full lifecycle of the etcd cluster. This means for example that by joining a new controller node with `k0s server "long-join-token"` k0s will automatically adjust the etcd cluster membership info to allow the new member to join the cluster.

**Note:** Currently k0s cannot shrink the etcd cluster. For now user needs to manually remove the etcd member and only after that shutdown the k0s controller on the removed node.

## Worker plane

![k0s worker processes](img/k0s_worker_processes.png)

Like for the control plane, k0s creates and manages the core worker components as naked processes on the worker node. Currently we support only [containerd](https://containerd.io) as the container engine.
