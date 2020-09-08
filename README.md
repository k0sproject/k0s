# MKE - Mirantis Kubernetes Engine

**Note:** The name of the project will probably change in near future.

MKE is yet another Kubernetes distro. Yes. But we do some of the things pretty differently than other distros out there.

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

Fetch official binaries (except `kine`, which is built from source):
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

### Control plane

```
mke server
```

This creates all the necessary certs and configs in `/var/lib/mke/pki`. Mke runs all control plane components in separate "naked" processes, does not depend on kubelet or container engine.

After control plane boots up, we need to create a join token for worker node:

```
mke token create --role=worker
```

*Note:* The token is super long atm, we intend to make it shorter at some point

### Worker node

Join a new worker node to the cluster by running:
```
mke worker "superlongtokenfrompreviousphase"
```

