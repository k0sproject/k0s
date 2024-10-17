# Architecture

**Note:** As k0s is a dynamic project, the product architecture may occasionally outpace the documentation. The high level concepts and patterns, however, should always apply.

## Packaging

The k0s package is a single, self-extracting binary that embeds Kubernetes binaries, the benefits of which include:

- Statically compiled
- No OS-level dependencies
- Requires no RPMs, dependencies, snaps, or any other OS-specific packaging
- Provides a single package for all operating systems
- Allows full version control for each dependency

![k0s packaging as a single binary](k0s_packaging.png)

## Control plane

As a single binary, k0s acts as the process supervisor for all other control plane components. As such, there is no container engine or kubelet running on controllers by default, which thus means that a cluster user cannot schedule workloads onto controller nodes.

![k0s Controller processes](k0s_controller_processes.png)

Using k0s you can create, manage, and configure each of the components, running each as a "naked" process. Thus, there is no container engine running on the controller node.

## Storage

Kubernetes control plane typically supports only etcd as the datastore. k0s, however, supports many other datastore options in addition to etcd, which it achieves by including [kine](https://github.com/k3s-io/kine). Kine allows the use of a wide variety of backend data stores, such as MySQL, PostgreSQL, SQLite, and dqlite (refer to the [`spec.storage` documentation](../configuration.md#specstorage)).

In the case of k0s managed etcd, k0s manages the full lifecycle of the etcd cluster. For example, by joining a new controller node with `k0s controller "long-join-token"` k0s  atomatically adjusts the etcd cluster membership info to allow the new member to join the cluster.

**Note**: k0s cannot shrink the etcd cluster. As such, to shut down the k0s controller on a node that node must first be manually removed from the etcd cluster.

## Worker node

![k0s worker processes](k0s_worker_processes.png)

As with the control plane, with k0s you can create and manage the core worker components as naked processes on the worker node.

By default, k0s workers use [containerd](https://containerd.io) as a high-level runtime and [runc](https://github.com/opencontainers/runc) as a low-level runtime. Custom runtimes are also supported, refer to [Using custom CRI runtimes](../runtime.md#using-custom-cri-runtimes).
