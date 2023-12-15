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

You find the config in `${DATADIR}/pki/admin.conf` (default: `/var/lib/k0s/pki/admin.conf`). Copy this file, and change the `localhost` entry to the public ip of the controller. Use the modified config to connect with kubectl:

```shell
export KUBECONFIG=/path/to/admin.conf
kubectl ...
```

## Why doesn't `kubectl get nodes` list the k0s controllers?

As a default, the control plane does not run kubelet at all, and will not accept any workloads, so the controller will not show up on the node list in kubectl. If you want your controller to accept workloads and run pods, you do so with:
`k0s controller --enable-worker` (recommended only as test/dev/POC environments).

## Is k0sproject really open source?

Yes, k0sproject is 100% open source. The source code is under Apache 2 and the documentation is under the Creative Commons License. Mirantis, Inc. is the main contributor and sponsor for this OSS project: building all the binaries from upstream, performing necessary security scans and calculating checksums so that it's easy and safe to use. The use of these ready-made binaries are subject to Mirantis EULA and the binaries include only open source software.
