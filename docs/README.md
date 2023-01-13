# k0s - The Zero Friction Kubernetes

![k0s logo](img/k0s-logo-full-color-light.svg)

k0s is an open source, all-inclusive Kubernetes distribution, which is configured with all of the features needed to build a Kubernetes cluster. Due to its simple design, flexible deployment options and modest system requirements, k0s is well suited for

- Any cloud
- Bare metal
- Edge and IoT

k0s drastically reduces the complexity of installing and running a CNCF certified Kubernetes distribution. With k0s new clusters can be bootstrapped in minutes and developer friction is reduced to zero. This allows anyone with no special skills or expertise in Kubernetes to easily get started.

k0s is distributed as a single binary with zero host OS dependencies besides the host OS kernel. It works with any Linux without additional software packages or configuration. Any security vulnerabilities or performance issues can be fixed directly in the k0s distribution that makes it extremely straightforward to keep the clusters up-to-date and secure.

## What happened to Github stargazers?

In September 2022 we made a human error while creating some build automation scripts&tools for the Github repository. Our automation accidentally changed the repo to a private one for few minutes. That itself is not a big deal and everything was restored quickly. But the nasty side effect is that it also removed all the stargazers at that point. :(

Before that mishap we had 4776 stargazers, making k0s one of the most popular Kubernetes distro out there.

**So if you are reading this, and have not yet starred [k0s repo](https://github.com/k0sproject/k0s/) we would highly appreciate the :star: to get our numbers closer to what they used to be.

## Key Features

- Certified and 100% upstream Kubernetes
- Multiple installation methods: [single-node](install.md), [multi-node](k0sctl-install.md), [airgap](airgap-install.md) and [Docker](k0s-in-docker.md)
- Automatic lifecycle management with k0sctl: [upgrade](upgrade.md), [backup and restore](backup.md)
- Modest [system requirements](system-requirements.md) (1 vCPU, 1 GB RAM)
- Available as a single binary with no [external runtime dependencies](external-runtime-deps.md) besides the kernel
- Flexible deployment options with [control plane isolation](networking.md#controller-worker-communication) as default
- Scalable from a single node to large, [high-available](high-availability.md) clusters
- Supports custom [Container Network Interface (CNI)](networking.md) plugins (Kube-Router is the default, Calico is offered as a preconfigured alternative)
- Supports custom [Container Runtime Interface (CRI)](runtime.md) plugins (containerd is the default)
- Supports all Kubernetes storage options with [Container Storage Interface (CSI)](storage.md), includes [OpenEBS host-local storage provider](storage.md#bundled-openebs-storage)
- Supports a variety of [datastore backends](configuration.md#specstorage): etcd (default for multi-node clusters), SQLite (default for single node clusters), MySQL, and PostgreSQL
- Supports x86-64, ARM64 and ARMv7
- Includes [Konnectivity service](networking.md#controller-worker-communication), CoreDNS and Metrics Server

## Getting Started

[Quick Start Guide](install.md) for creating a full Kubernetes cluster with a single node.

## Demo

![k0s demo](img/k0s_demo.gif)

## Community Support

- [Community Slack](https://join.slack.com/t/k8slens/shared_invite/zt-wcl8jq3k-68R5Wcmk1o95MLBE5igUDQ) - Request for support and help from the k0s community via Slack (shared Slack channel with Lens).
- [GitHub Issues](https://github.com/k0sproject/k0s/issues) - Submit your issues and feature requests via GitHub.

We welcome your help in building k0s! If you are interested, we invite you to check out the [Contributing Guide](contributors/overview.md) and the [Code of Conduct](contributors/CODE_OF_CONDUCT.md).

## Commercial Support

[Mirantis](https://www.mirantis.com/software/k0s/) offers technical support, professional services and training for k0s. The support subscriptions include, for example, prioritized support (Phone, Web, Email) and access to verified extensions on top of your k0s cluster.

For any k0s inquiries, please contact us via email [info@k0sproject.io](mailto:info@k0sproject.io).
