# MKE - Mirantis Kubernetes Engine

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

Build mke without any embedded binaries (requires that kubernets
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

This create all the necessary certs and configs in `/var/lib/mke/pki`. Mke runs all control plane components in separate "naked" processes, does not depend on kubelet or container engine.

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

