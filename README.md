![Go build](https://github.com/k0sproject/k0s/workflows/Go%20build/badge.svg) ![k0s network conformance](https://github.com/k0sproject/k0s/workflows/k0s%20Check%20Network/badge.svg)

![GitHub release (latest by date)](https://img.shields.io/github/v/release/k0sproject/k0s?label=latest%20stable%20release) ![GitHub release (latest SemVer including pre-releases)](https://img.shields.io/github/v/release/k0sproject/k0s?include_prereleases&label=latest%20pre-release) ![GitHub commits since latest release (by date)](https://img.shields.io/github/commits-since/k0sproject/k0s/latest) ![GitHub Repo stars](https://img.shields.io/github/stars/k0sproject/k0s?color=blueviolet&label=Stargazers)


# k0s - Zero Friction Kubernetes

![k0s logo](k0s-logo-full-color.svg)

k0s is an all-inclusive Kubernetes distribution with all the required bells and whistles preconfigured to make building a Kubernetes clusters a matter of just copying an executable to every host and running it.

## Key Features

- Packaged as a single static binary
- Self-hosted, isolated control plane
- Variety of storage backends: etcd, SQLite, MySQL (or any compatible), PostgreSQL
- Elastic control-plane
- Vanilla upstream Kubernetes
- Supports custom container runtimes (containerd is the default)
- Supports custom Container Network Interface (CNI) plugins (calico is the default)
- Supports x86_64 and arm64

![k0s demo](k0s_demo.gif)

## Motivation

**Note:** Some of these goals are not 100% fulfilled yet.

_We have seen a gap between the host OS and Kubernetes that runs on top of it: How to ensure they work together as they are upgraded independent from each other? Who’s  responsible for vulnerabilities or performance issues originating from the host OS that affect the K8S on top?_

**&rarr;** k0s Kubernetes is fully self contained. It’s distributed as a single binary with no host OS deps besides the kernel. Any vulnerability or perf issues may be fixed in k0s Kubernetes.

_We have seen K8S with partial FIPS security compliance: How to ensure security compliance for critical applications if only part of the system is FIPS compliant?_

**&rarr;** k0s Kubernetes core + all included host OS dependencies + components on top may be compiled and packaged as a 100% FIPS compliant distribution with a proper toolchain.

_We have seen Kubernetes with cumbersome lifecycle management, high minimum system requirements, weird host OS and infra restrictions, and/or need to use different distros to meet different use cases._

**&rarr;** k0s Kubernetes is designed to be lightweight at its core. It comes with a tool to automate cluster lifecycle management. It works on any host OS and infrastructure, and may be extended to work with any use cases such as edge, IoT, telco, public clouds, private data centers, and hybrid & hyper converged cloud applications without sacrificing the pure Kubernetes compliance or amazing developer experience.



## Other Features

- Kubernetes 1.19
- Containerd 1.4
- Control plane storage options:
  - sqlite (in-cluster)
  - etcd (in-cluster, managed, default)
  - mysql (external)
  - postgresql (external)
- CNI providers
  - Calico 3.16 (default)
  - Custom (bring-your-own)
- Control plane isolation:
  - fully isolated (default)
  - tainted worker
- Control plane - node communication
  - Konnectivity service (default)
- CoreDNS 1.7
- Metrics-server 0.3
- Custom roles\profiles for worker nodes

See more in [architecture docs](docs/architecture.md)

## Status

We're still on the 0.x.y release versions, so things are not yet 100% stable. That includes both stability of different APIs and config structures as well as the stability of k0s itself. While we do have some basic smoke testing happening we're still lacking more longer running stability testing for k0s based clusters. And of course we only test some known configuration combinations.

With the help of community we're hoping to push for 1.0.0 release out in early 2021.

## Scope

While some Kubernetes distros package everything and the kitchen sink in, k0s tries to minimize the amount of "add-ons" to bundle in. Instead, we aim to provide robust and versatile "base" for running Kubernetes in various setups. Of course we will provide some ways to easily control and setup various "add-ons" but we will likely not bundle many of those into k0s itself. There's couple reasons why we think this is the correct way:
- Many of the addons such as ingresses, service meshes, storage etc. are VERY opinionated. We try to build this base with less opinions. :D
- Keeping up with the upstream releases with many external addons is very maintenance heavy. Shipping with old versions does not make much sense either.

With strong enough arguments we might take in new addons but in general those should be something that are essential for the "core" of k0s.

## Cluster bootstrapping

Move the built `k0s` binary to each of the nodes.

```
k0s server
```

This creates all the necessary certs and configs in `/var/lib/k0s/pki`. k0s runs all control plane components in separate "naked" processes, does not depend on kubelet or container engine.

After control plane boots up, we need to create a join token for worker node:

```
k0s token create --role=worker
```

Join a new worker node to the cluster by running:
```
k0s worker "superlongtokenfrompreviousphase"
```

For more detailed description see [creating cluster documentation](docs/create-cluster.md).

### k0s-in-docker

**Note:** Running k0s, or any other Kubernetes distro, like this is not really a production ready setup. :)

To run a single node controller+worker combo, just run it in docker with:
```
docker run -d --name k0s-controller --hostname controller --privileged -v /var/lib/k0s -p 6443:6443 docker.pkg.github.com/k0sproject/k0s/k0s:<version>
```

Replace `<version>` with a released version number, we build the image for all tagged releases.

That's it, in a minute or so there should be controller+worker combo running in the container.

Just grab the kubeconfig with `docker exec k0s-controller cat /var/lib/k0s/pki/admin.conf` and paste e.g. into [Lens](https://k8slens.dev/). ;)

Read more details at [running k0s in Docker](docs/k0s-in-docker.md).

## Build

`k0s` can be built in 3 different ways:

Fetch official binaries (except `kine` and `konnectivity-server`, which are built from source):
```
make EMBEDDED_BINS_BUILDMODE=fetch
```

Build Kubernetes components from source as static binaries (requires docker):
```
make EMBEDDED_BINS_BUILDMODE=docker
```

Build k0s without any embedded binaries (requires that Kubernetes
binaries are pre-installed on the runtime system):
```
make EMBEDDED_BINS_BUILDMODE=none
```

Builds can be done in parallel:
```
make -j$(nproc)
```

## Smoke test

To run a smoke test after build:
```
make check-basic
```

