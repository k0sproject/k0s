# k0s - The Zero Friction Kubernetes by Team Lens

<!-- When changing this file, consider to change docs/README.md, too! -->

[![Go build](https://github.com/k0sproject/k0s/actions/workflows/go.yml/badge.svg?branch=main)](https://github.com/k0sproject/k0s/actions/workflows/go.yml?query=branch%3Amain)
[![k0s network conformance](https://github.com/k0sproject/k0s/workflows/k0s%20Check%20Network/badge.svg)](https://github.com/k0sproject/k0s/actions/workflows/check-network.yaml)
[![Go Reference](https://img.shields.io/badge/code%20reference-go.dev-bc42f5.svg)](https://pkg.go.dev/github.com/k0sproject/k0s)
![GitHub Repo stars](https://img.shields.io/github/stars/k0sproject/k0s?color=blueviolet&label=Stargazers)
[![Releases](https://img.shields.io/github/downloads/k0sproject/k0s/total.svg)](https://github.com/k0sproject/k0s/tags?label=Downloads)

![GitHub release (latest by date)](https://img.shields.io/github/v/release/k0sproject/k0s?label=latest%20stable)
![GitHub release (latest SemVer including pre-releases)](https://img.shields.io/github/v/release/k0sproject/k0s?include_prereleases&label=latest-release%20%28including+pre-release%29) ![GitHub commits since latest release (by date)](https://img.shields.io/github/commits-since/k0sproject/k0s/latest)

![k0s-logo-dark](docs/img/k0s-logo-full-color-dark.svg#gh-dark-mode-only)
![k0s-logo-light](docs/img/k0s-logo-full-color-light.svg#gh-light-mode-only)

## What happened to GitHub stargazers?

In September 2022 we made a human error while creating some build automation scripts&tools for the GitHub repository. Our automation accidentally changed the repo to a private one for few minutes. That itself is not a big deal and everything was restored quickly. But the nasty side effect is that it also removed all the stargazers at that point. :(

Before that mishap we had 4776 stargazers, making k0s one of the most popular Kubernetes distro out there.

**So if you are reading this, and have not yet starred this repo we would highly appreciate the :star: to get our numbers closer to what they used to be.**

## The Repository

This repository ("k0s") is where Team Lens develops the [k0s](https://k8slens.dev/kubernetes) product together with the community. It is backed by a number of Kubernetes and cloud native ecosystem pioneers. This source code is available to everyone under the [Apache License 2.0](./LICENSE).

<!-- Start Overview -->
## Overview

k0s is an open source, all-inclusive Kubernetes distribution, which is configured with all of the features needed to build a Kubernetes cluster and packaged as a single binary for ease of use. Due to its simple design, flexible deployment options and modest system requirements, k0s is well suited for

- Any cloud
- Bare metal
- Edge and IoT

k0s drastically reduces the complexity of installing and running a CNCF certified Kubernetes distribution. With k0s new clusters can be bootstrapped in minutes and developer friction is reduced to zero. This allows anyone with no special skills or expertise in Kubernetes to easily get started.

k0s is distributed as a single binary with zero host OS dependencies besides the host OS kernel. It works with any Linux without additional software packages or configuration. Any security vulnerabilities or performance issues can be fixed directly in the k0s distribution that makes it extremely straightforward to keep the clusters up-to-date and secure.
<!-- End Overview -->

<!-- Start Key Features -->
## Key Features

- Certified and 100% upstream Kubernetes
- Multiple installation methods: [single-node](docs/install.md), [multi-node](docs/k0sctl-install.md), [airgap](docs/airgap-install.md) and [Docker](docs/k0s-in-docker.md)
- Automatic lifecycle management with k0sctl: [upgrade](docs/upgrade.md), [backup and restore](docs/backup.md)
- Modest [system requirements](docs/system-requirements.md) (1 vCPU, 1 GB RAM)
- Available as a single binary with no [external runtime dependencies](docs/external-runtime-deps.md) besides the kernel
- Flexible deployment options with [control plane isolation](docs/networking.md#controller-worker-communication) as default
- Scalable from a single node to large, [high-available](docs/high-availability.md) clusters
- Supports custom [Container Network Interface (CNI)](docs/networking.md) plugins (Kube-Router is the default, Calico is offered as a preconfigured alternative)
- Supports custom [Container Runtime Interface (CRI)](docs/runtime.md) plugins (containerd is the default)
- Supports all Kubernetes storage options with [Container Storage Interface (CSI)](docs/storage.md)
- Supports a variety of [datastore backends](docs/configuration.md#specstorage): etcd (default for multi-node clusters), SQLite (default for single node clusters), MySQL, and PostgreSQL
- Supports x86-64, ARM64 and ARMv7
- Includes [Konnectivity service](docs/networking.md#controller-worker-communication), CoreDNS and Metrics Server
<!-- End Key Features -->

## Getting Started

If you'd like to try k0s, please jump in to our:

- [Quick Start Guide](https://docs.k0sproject.io/stable/install/) - Create a full Kubernetes cluster with a single node that includes both the controller and the worker.
- [Install using k0sctl](https://docs.k0sproject.io/stable/k0sctl-install/) - Deploy and upgrade multi-node clusters with one command.
- [NanoDemo](https://docs.k0sproject.io/stable/#demo) - Watch a .gif recording on how to create a k0s instance.
- [Run k0s in Docker](https://docs.k0sproject.io/stable/k0s-in-docker/) - Run k0s controllers and workers in containers.
- For docs, tutorials, and other k0s resources, see [docs main page](https://docs.k0sproject.io).

<!-- Start Join the Community -->
## Join the Community

- [Lens Forums] - Request for support and help from the Lens and k0s community.
- [GitHub Issues] - Submit your issues and feature requests via GitHub.

We welcome your help in building k0s! If you are interested, we invite you to
check out the [Contributing Guide] and the [Code of Conduct].

[Lens Forums]: https://forums.k8slens.dev/
[GitHub Issues]: https://github.com/k0sproject/k0s/issues
[Contributing Guide]: https://docs.k0sproject.io/stable/contributors/overview/
[Code of Conduct]:https://docs.k0sproject.io/stable/contributors/CODE_OF_CONDUCT/

### Community hours

We will be holding regular community hours. Everyone in the community is welcome to drop by and ask questions, talk about projects, and chat.

We currently have a monthly office hours call on the last Tuesday of the month.

To see the call details in your local timezone, check out [https://dateful.com/eventlink/2735919704](https://dateful.com/eventlink/2735919704).

### Adopters

k0s is used in numerous cases and environments. The range we've seen it being adopted ranges from very small far edge deployments to big data center deployments. Please add your use case to [adopters] list.

[adopters]: docs/adopters.md
<!-- End Join the Community -->

<!-- Start Motivation -->
## Motivation

_We have seen a gap between the host OS and Kubernetes that runs on top of it: How to ensure they work together as they are upgraded independent from each other? Who is responsible for vulnerabilities or performance issues originating from the host OS that affect the K8S on top?_

**&rarr;** k0s is fully self contained. It’s distributed as a single binary with no host OS deps besides the kernel. Any vulnerability or perf issues may be fixed in k0s Kubernetes.

_We have seen K8S with partial FIPS security compliance: How to ensure security compliance for critical applications if only part of the system is FIPS compliant?_

**&rarr;** k0s core + all included host OS dependencies + components on top may be compiled and packaged as a 100% FIPS compliant distribution using a proper toolchain.

_We have seen Kubernetes with cumbersome lifecycle management, high minimum system requirements, weird host OS and infra restrictions, and/or need to use different distros to meet different use cases._

**&rarr;** k0s is designed to be lightweight at its core. It comes with a tool to automate cluster lifecycle management. It works on any host OS and infrastructure, and may be extended to work with any use cases such as edge, IoT, telco, public clouds, private data centers, and hybrid & hyper converged cloud applications without sacrificing the pure Kubernetes compliance or amazing developer experience.
<!-- End Motivation -->

## Status

k0s is ready for production (starting from v1.21.0+k0s.0). Since the initial release of k0s back in November 2020, we have made numerous releases, improved stability, added new features, and most importantly, listened to our users and community in an effort to create the most modern Kubernetes product out there. The active development continues to make k0s even better.

<!-- Start Scope -->
## Scope

While some Kubernetes distros package everything and the kitchen sink, k0s tries to minimize the amount of "add-ons" to bundle in. Instead, we aim to provide a robust and versatile "base" for running Kubernetes in various setups. Of course we will provide some ways to easily control and setup various "add-ons", but we will not bundle many of those into k0s itself. There are a couple of reasons why we think this is the correct way:

- Many of the addons such as ingresses, service meshes, storage etc. are VERY opinionated. We try to build this base with fewer opinions. :D
- Keeping up with the upstream releases with many external addons is very maintenance heavy. Shipping with old versions does not make much sense either.

With strong enough arguments we might take in new addons, but in general those should be something that are essential for the "core" of k0s.
<!-- End Scope -->

## Build

The requirements for building k0s from source are as follows:

- GNU Make (v3.81 or newer)
- coreutils
- findutils
- Docker

All of the compilation steps are performed inside Docker containers, no
installation of Go is required.

The k0s binary can be built in two different ways:

The "k0s" way, self-contained, all binaries compiled from source, statically
linked and embedded:

```shell
make
```

The "package maintainer" way, without any embedded binaries (requires that the
required binaries are provided separately at runtime):

```shell
make EMBEDDED_BINS_BUILDMODE=none
```

The embedded binaries can be built on their own:

```shell
make -C embedded-bins
```

Builds can be done in parallel:

```shell
make -j$(nproc)
```

## Smoke test

Additionally to the requirements for building k0s, the smoke tests _do_ require
a local Go installation. you can run `./vars.sh go_version` in a terminal to
find out the version that's being used to build k0s. It will print the
corresponding Go version to stdout.

To run a basic smoke test after build:

```shell
make check-basic
```
