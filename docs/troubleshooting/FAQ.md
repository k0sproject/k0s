<!--
SPDX-FileCopyrightText: 2020 k0s authors
SPDX-License-Identifier: CC-BY-SA-4.0
-->

# Frequently asked questions

## How is k0s pronounced?

kay-zero-ess

## How do I run a single node cluster?

The cluster can be started with:

```shell
k0s controller --single
```

See also the [Getting Started](https://docs.k0sproject.io/stable/install/) tutorial.

## How do I connect to the cluster?

You find the config in `${DATADIR}/pki/admin.conf` (default: `/var/lib/k0s/pki/admin.conf`). Copy this file, and change the `localhost` entry to the public IP address of the controller. Use the modified config to connect with kubectl:

```shell
export KUBECONFIG=/path/to/admin.conf
kubectl ...
```

## Why doesn't `kubectl get nodes` list the k0s controllers?

As a default, the control plane does not run kubelet at all, and will not accept any workloads, so the controller will not show up on the node list in kubectl. If you want your controller to accept workloads and run pods, you do so with:
`k0s controller --enable-worker` (recommended only as test/dev/POC environments).

## Is k0s really open source?

Yes, k0s is 100% open source. The source code is licensed under the Apache 2.0
License, and the documentation under a Creative Commons License. The project is
part of the [CNCF Sandbox]. While Mirantis, Inc. remains a principal contributor
and sponsor, k0s adheres to CNCF's open governance and IP policies to ensure
transparency and community-driven development under a vendor-neutral umbrella.

[CNCF Sandbox]: https://www.cncf.io/sandbox-projects/

## A kubeconfig created via [`k0s kubeconfig`](../cli/k0s_kubeconfig.md) has been leaked, what can I do?

Kubernetes does not support certificate revocation (see [k/k/18982]). This means
that you cannot disable the leaked credentials. The only way to effectively
revoke them is to [replace the Kubernetes CA] for your cluster.

[k/k/18982]: https://github.com/kubernetes/kubernetes/issues/18982
[replace the Kubernetes CA]: certificate-authorities.md#replacing-the-kubernetes-ca-and-sa-key-pair
