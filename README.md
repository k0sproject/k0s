# MKE - Mirantis Kubernetes Engine

**Note:** The name of the project will probably change in near future.

MKE is yet another Kubernetes distro. Yes. But we do some of the things pretty differently than other distros out there.

## Motivation

**Note:** Some of these goals are not 100% fullfilled as of yet.

_We have seen a gap between host OS and K8S that runs on top: How to ensure they work together as they are upgraded independent from each other? Who’s in responsible for vulnerabilities or performance issues originating from host OS but affecting K8S on top?_

**&rarr;** MKE K8S is fully self contained. It’s distributed as single binary with no host OS deps besides the kernel. Any vulnerability or perf issues may be fixed in MKE K8S.

_We have seen K8S with partial FIPS security compliance: How to ensure security compliance for critical applications if only part of the system if FIPS compliant?_

**&rarr;** MKE K8S core + all included host OS dependencies + components on top may be compiled and packaged as 100% FIPS compliant distribution with proper toolchain.

_We have seen K8S with cumbersome lifecycle management, high minimum system requirements, weird host OS and infra restrictions, and/or need to use different distros to meet different use cases._

**&rarr;** MKE K8S is designed to be ultra lightweight at its core. It comes with a tool to automate cluster lifecycle management. It works on any host OS and infrastructure, and may be extended to work with any use cases such as edge, IoT, telco, public clouds, private data centers, and hybrid & hyper converged cloud applications without sacrificing the pure K8S compliance or amazing developer experience.


Some of the high level goals of the project:
- Packaged as single binary
- Self-hosted, isolated control plane
- Variety of storage backends: etcd, SQLite, MySQL (or any compatible), PostgreSQL
- Elastic control-plane
- Vanilla upstream Kubernetes

See more in [architecture docs](docs/architecture.md)

## Features

- One static binary
- Kubernetes 1.19
- Containerd 1.4
- Control plane storage options:
  - sqlite (in-cluster, default)
  - etcd (in-cluster, managed)
  - mysql (external)
  - postgresql (external)
- CNI providers
  - Calico 3.15 (default)
  - Custom (bring-your-own)
- Control plane isolation:
  - fully isolated (default)
  - tainted worker
- Control plane - node communication
  - Konnectivity service (default)
- CoreDNS 1.7
- Metrics-server 0.3

## Build

`mke` can be built in 3 different ways:

Fetch official binaries (except `kine` and `konnectivity-server`, which are built from source):
```
make EMBEDDED_BINS_BUILDMODE=fetch
```

Build kubernetes components from source as static binaries (requires docker):
```
make EMBEDDED_BINS_BUILDMODE=docker
```

Build mke without any embedded binaries (requires that kubernetes
binaries are pre-installed on the runtime system):
```
make EMBEDDED_BINS_BUILDMODE=none
```

Builds can be done in parallel:
```
make -j$(nproc)
```

## Cluster bootstrapping

Currently mke makes only very basic default (hardcoded) configs for everything.

Move the built `mke` binary to each of the nodes.

```
mke server
```

This creates all the necessary certs and configs in `/var/lib/mke/pki`. Mke runs all control plane components in separate "naked" processes, does not depend on kubelet or container engine.

After control plane boots up, we need to create a join token for worker node:

```
mke token create --role=worker
```

Join a new worker node to the cluster by running:
```
mke worker "superlongtokenfrompreviousphase"
```

For more detailed description see [creating cluster documentation](docs/create-cluster.md) 

