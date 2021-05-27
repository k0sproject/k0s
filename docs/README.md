# Overview

![k0s logo](img/k0s-logo-full-color.svg)

k0s is an all-inclusive Kubernetes distribution, configured with all of the features needed to build a Kubernetes cluster simply by copying and running an executable file on each target host.

## Key Features

- Available as a single static binary
- Offers a self-hosted, isolated control plane
- Supports a variety of storage backends, including etcd, SQLite, MySQL (or any compatible), and PostgreSQL.
- Offers an Elastic control plane
- Vanilla upstream Kubernetes
- Supports custom container runtimes (containerd is the default)
- Supports custom Container Network Interface (CNI) plugins (calico is the default)
- Supports x86_64 and arm64

## Join the Community

We welcome your help in building k0s! If you are interested, we invite you to check out the [k0s Contributing Guide](contributors/overview.md) and our [Code of Conduct](contributors/CODE_OF_CONDUCT.md).

## Demo

![k0s demo](img/k0s_demo.gif)

## Downloading k0s

[Download k0s](https://github.com/k0sproject/k0s/releases) for linux amd64 and arm64 architectures.

## Getting Started

[Quick Start Guide](install.md) for creating a full Kubernetes cluster with a single node.