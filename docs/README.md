# Overview

![k0s logo](img/k0s-logo-full-color-light.svg)

[k0s](https://k8slens.dev/kubernetes) is an all-inclusive Kubernetes distribution, which is configured with all of the features needed to build a Kubernetes cluster and packaged as a single binary for ease of use.

k0s fits well in any cloud environment, but can also be used in IoT gateways, Edge and Bare metal deployments due to its simple design, flexible deployment options and modest system requirements.

## Key Features

- Different installation methods: [single-node](install.md), [multi-node](k0sctl-install.md), [airgap](airgap-install.md) and [Docker](k0s-in-docker.md)
- Automatic lifecycle management with k0sctl: [upgrade](upgrade.md), [backup and restore](backup.md)
- Modest [system requirements](system-requirements.md) (1 vCPU, 1 GB RAM)
- Vanilla upstream Kubernetes (with no changes)
- Available as a single binary with no [OS dependencies](os-deps.md) besides the kernel
- Flexible deployment options with [control plane isolation](networking.md#controller-worker-communication) as default
- Scalable from a single node to large, [high-available](high-availability.md) clusters
- Supports custom [Container Network Interface (CNI)](networking.md) plugins (Kube-Router is the default, Calico is offered as preconfigured alternative)
- Supports custom [Container Runtime Interface (CRI)](runtime.md) plugins (containerd is the default)
- Supports all Kubernetes storage options with [Container Storage Interface (CSI)](storage.md)
- Supports a variety of [datastore backends](configuration.md#specstorage): etcd (default for multi-node clusters), SQLite (default for single node clusters), MySQL, and PostgreSQL
- Supports x86-64, ARM64 and ARMv7
- [Konnectivity service](networking.md#controller-worker-communication), CoreDNS, Metrics Server

## Getting Started

[Quick Start Guide](install.md) for creating a full Kubernetes cluster with a single node.

## Demo

![k0s demo](img/k0s_demo.gif)

## Community Support

- [Community Slack](https://join.slack.com/t/k8slens/shared_invite/zt-wcl8jq3k-68R5Wcmk1o95MLBE5igUDQ) - Request for support and help from the k0s community via Slack (shared Slack channel with Lens).
- [Github Issues](https://github.com/k0sproject/k0s/issues) - Submit your issues and feature requests via Github.

We welcome your help in building k0s! If you are interested, we invite you to check out the [Contributing Guide](contributors/overview.md) and the [Code of Conduct](contributors/CODE_OF_CONDUCT.md).

## Commercial Support

[Mirantis](https://www.mirantis.com/software/k0s/) offers technical support, professional services and training for k0s. The support subscriptions include for example prioritized support (Phone, Web, Email) and access to verified extensions on top of your k0s cluster.

For any k0s inquiries, please contact us via email [info@k0sproject.io](mailto:info@k0sproject.io).
