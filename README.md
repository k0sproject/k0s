[![Go build](https://github.com/k0sproject/k0s/actions/workflows/go.yml/badge.svg?branch=main)](https://github.com/k0sproject/k0s/actions/workflows/go.yml?query=branch%3Amain)
![k0s network conformance](https://github.com/k0sproject/k0s/workflows/k0s%20Check%20Network/badge.svg)
[![Slack](https://img.shields.io/badge/join%20slack-%23k0s-4A154B.svg)](https://join.slack.com/t/k8slens/shared_invite/zt-wcl8jq3k-68R5Wcmk1o95MLBE5igUDQ)
[![Go Reference](https://img.shields.io/badge/code%20reference-go.dev-bc42f5.svg)](https://pkg.go.dev/github.com/k0sproject/k0s)
![GitHub release (latest by date)](https://img.shields.io/github/v/release/k0sproject/k0s?label=latest%20stable) ![GitHub release (latest SemVer including pre-releases)](https://img.shields.io/github/v/release/k0sproject/k0s?include_prereleases&label=latest-release%20%28including+pre-release%29) ![GitHub commits since latest release (by date)](https://img.shields.io/github/commits-since/k0sproject/k0s/latest)

![GitHub Repo stars](https://img.shields.io/github/stars/k0sproject/k0s?color=blueviolet&label=Stargazers) [![Releases](https://img.shields.io/github/downloads/k0sproject/k0s/total.svg)](https://github.com/k0sproject/k0s/tags?label=Downloads)

# k0s - Zero Friction Kubernetes

![k0s-logo-dark](docs/img/k0s-logo-full-color-dark.svg#gh-dark-mode-only)
![k0s-logo-light](docs/img/k0s-logo-full-color-light.svg#gh-light-mode-only)

[k0s](https://k0sproject.io/) is an all-inclusive Kubernetes distribution, which is configured with all of the features needed to build a Kubernetes cluster and packaged as a single binary for ease of use.

k0s fits well in any cloud environment, but can also be used in IoT gateways, Edge and Bare metal deployments due to its simple design, flexible deployment options and modest system requirements.

## Key Features

- Different installation methods: [single-node](docs/install.md), [multi-node](docs/k0sctl-install.md), [airgap](docs/airgap-install.md) and [Docker](docs/k0s-in-docker.md)
- Automatic lifecycle management with k0sctl: [upgrade](docs/upgrade.md), [backup and restore](docs/backup.md)
- Modest [system requirements](docs/system-requirements.md) (1 vCPU, 1 GB RAM)
- Vanilla upstream Kubernetes (with no changes)
- Available as a single binary with no [OS dependencies](docs/os-deps.md) besides the kernel
- Flexible deployment options with [control plane isolation](docs/networking.md#controller-worker-communication) as default
- Scalable from a single node to large, [high-available](docs/high-availability.md) clusters
- Supports custom [Container Network Interface (CNI)](docs/networking.md) plugins (Kube-Router is the default, Calico is offered as preconfigured alternative)
- Supports custom [Container Runtime Interface (CRI)](docs/runtime.md) plugins (containerd is the default)
- Supports all Kubernetes storage options with [Container Storage Interface (CSI)](docs/storage.md)
- Supports a variety of [datastore backends](docs/configuration.md#specstorage): etcd (default for multi-node clusters), SQLite (default for single node clusters), MySQL, and PostgreSQL
- Supports x86-64, ARM64 and ARMv7
- [Konnectivity service](docs/networking.md#controller-worker-communication), CoreDNS, Metrics Server

## Getting Started

If you'd like to try k0s, please jump in to our:

- [Quick Start Guide](https://docs.k0sproject.io/latest/install/) - Create a full Kubernetes cluster with a single node that includes both the controller and the worker.
- [Install using k0sctl](https://docs.k0sproject.io/latest/k0sctl-install/) - Deploy and upgrade multi-node clusters with one command.
- [NanoDemo](https://docs.k0sproject.io/latest/#demo) - Watch a .gif recording on how to create a k0s instance.
- [Run k0s in Docker](https://docs.k0sproject.io/latest/k0s-in-docker/) - Run k0s controllers and workers in containers.
- For docs, tutorials, and other k0s resources, see [docs main page](https://docs.k0sproject.io).

## Join the Community

- [Community Slack](https://join.slack.com/t/k8slens/shared_invite/zt-wcl8jq3k-68R5Wcmk1o95MLBE5igUDQ) - Request for support and help from the k0s community via Slack (shared Slack channel with Lens).
- [Github Issues](https://github.com/k0sproject/k0s/issues) - Submit your issues and feature requests via Github.

We welcome your help in building k0s! If you are interested, we invite you to check out the [Contributing Guide](https://docs.k0sproject.io/latest/contributors/overview/) and the [Code of Conduct](https://docs.k0sproject.io/latest/contributors/CODE_OF_CONDUCT/).

## Motivation

_We have seen a gap between the host OS and Kubernetes that runs on top of it: How to ensure they work together as they are upgraded independent from each other? Who is responsible for vulnerabilities or performance issues originating from the host OS that affect the K8S on top?_

**&rarr;** k0s is fully self contained. It’s distributed as a single binary with no host OS deps besides the kernel. Any vulnerability or perf issues may be fixed in k0s Kubernetes.

_We have seen K8S with partial FIPS security compliance: How to ensure security compliance for critical applications if only part of the system is FIPS compliant?_

**&rarr;** k0s core + all included host OS dependencies + components on top may be compiled and packaged as a 100% FIPS compliant distribution using a proper toolchain.

_We have seen Kubernetes with cumbersome lifecycle management, high minimum system requirements, weird host OS and infra restrictions, and/or need to use different distros to meet different use cases._

**&rarr;** k0s is designed to be lightweight at its core. It comes with a tool to automate cluster lifecycle management. It works on any host OS and infrastructure, and may be extended to work with any use cases such as edge, IoT, telco, public clouds, private data centers, and hybrid & hyper converged cloud applications without sacrificing the pure Kubernetes compliance or amazing developer experience.

## Status

k0s is ready for production (starting from v1.21.0+k0s.0). Since the initial release of k0s back in November 2020, we have made numerous releases, improved stability, added new features, and most importantly, listened to our users and community in an effort to create the most modern Kubernetes product out there. The active development continues to make k0s even better.

## Scope

While some Kubernetes distros package everything and the kitchen sink, k0s tries to minimize the amount of "add-ons" to bundle in. Instead, we aim to provide a robust and versatile "base" for running Kubernetes in various setups. Of course we will provide some ways to easily control and setup various "add-ons", but we will not bundle many of those into k0s itself. There are a couple of reasons why we think this is the correct way:

- Many of the addons such as ingresses, service meshes, storage etc. are VERY opinionated. We try to build this base with fewer opinions. :D
- Keeping up with the upstream releases with many external addons is very maintenance heavy. Shipping with old versions does not make much sense either.

With strong enough arguments we might take in new addons, but in general those should be something that are essential for the "core" of k0s.

## Build

`k0s` can be built in two different ways:

Build Kubernetes components from source as static binaries (requires docker):

```shell
make EMBEDDED_BINS_BUILDMODE=docker
```

Build k0s without any embedded binaries (requires that Kubernetes binaries are pre-installed on the runtime system):

```shell
make EMBEDDED_BINS_BUILDMODE=none
```

Builds can be done in parallel:

```shell
make -j$(nproc)
```

## Smoke test

To run a smoke test after build:

```shell
make check-basic
```
