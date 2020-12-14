# Overview
![k0s logo](img/k0s-logo-full-color.svg)

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

## Join the Community
If you'd like to help build k0s, please check out our guide to [Contributing](contributors/overview.md) and our [Code of Conduct](contributors/CODE_OF_CONDUCT.md).

## Demo
![k0s demo](img/k0s_demo.gif)

## Downloading k0s
[Download k0s](https://github.com/k0sproject/k0s/releases) for linux amd64 and arm64 architectures.

## Quick start
[Creating A k0s Cluster](create-cluster.md)