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

## Try k0s

If you'd like to try k0s, please jump to our:

- [Super QuickStart](#super_quickstart) - Create a k0s control plane and worker, and access it locally with kubectl.
- [NanoDemo](#nanodemo) - Watch a .gif recording of how to create a k0s instance.  
- [Better QuickStart](#better_quickstart) - Create a k0s control plane and worker, then access it with Lens from anywhere.
- [Create a k0s cluster](https://github.com/k0sproject/k0s/blob/main/docs/create-cluster.md) - For when you're ready to build a multi-node cluster.

You may also be interested in [current version specifications](#version_specs). For docs, tutorials, and other k0s resources, see our [Docs and Resources](https://github.com/k0sproject/k0s/tree/main/docs/docs_resources_TOC) mainpage.

## Join the Community

If you'd like to help build k0s, please check out our guide to [Contributing](https://github.com/k0sproject/k0s/tree/main/CONTRIBUTING.md) and our [Code of Conduct](https://github.com/k0sproject/k0s/tree/main/CODE_OF_CONDUCT.md).

## What's Our Motivation?

Why are we building k0s? Several reasons:

**Note:** Some of these goals are not 100% fulfilled yet.

_We have seen a gap between the host OS and Kubernetes that runs on top of it: How to ensure they work together as they are upgraded independent from each other? Who’s  responsible for vulnerabilities or performance issues originating from the host OS that affect the K8S on top?_

**&rarr;** k0s Kubernetes is fully self contained. It’s distributed as a single binary with no host OS dependencies besides the kernel. Any vulnerability or performance issues can be fixed in k0s Kubernetes.

_We have seen K8S with partial FIPS security compliance: How to ensure security compliance for critical applications if only part of the system is FIPS compliant?_

**&rarr;** k0s Kubernetes core + all included host OS dependencies + components on top may be compiled and packaged as a 100% FIPS-compliant distribution with a proper toolchain.

_We have seen Kubernetes with cumbersome lifecycle management, high minimum system requirements, weird host OS and infra restrictions, and/or need to use different distros to meet different use cases._

**&rarr;** k0s Kubernetes is designed to be lightweight at its core. It comes with a tool to automate cluster lifecycle management. It works on any host OS and infrastructure, and may be extended to work with any use cases such as edge, IoT, telco, public clouds, private data centers, and hybrid & hyper converged cloud applications without sacrificing the pure Kubernetes compliance or amazing developer experience.

## Status

We're still on the 0.x.y release versions, so things are not yet 100% stable. That includes both stability of different APIs and config structures as well as the stability of k0s itself. While we do have some basic smoke testing happening we're still lacking more longer running stability testing for k0s based clusters. And of course we only test some known configuration combinations.

With the help of community we're hoping to push for 1.0.0 release out in early 2021.

## Scope

While some Kubernetes distros package everything and the kitchen sink in, k0s tries to minimize the amount of "add-ons" to bundle in. Instead, we aim to provide robust and versatile "base" for running Kubernetes in various setups. Of course we will provide some ways to easily control and setup various "add-ons" but we will likely not bundle many of those into k0s itself. There's couple reasons why we think this is the correct way:
- Many of the addons such as ingresses, service meshes, storage etc. are VERY opinionated. We try to build this base with less opinions. :D
- Keeping up with the upstream releases with many external addons is very maintenance heavy. Shipping with old versions does not make much sense either.

With strong enough arguments we might take in new addons but in general those should be something that are essential for the "core" of k0s.

<a name="version_specs"></a>
## Current Specs

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

See more in the [architecture doc](docs/architecture.md) and in architectural resources linked in our [Docs and Resources](https://github.com/k0sproject/k0s/tree/main/docs/docs_resources_TOC) mainpage.

<a name="super_quickstart"></a>
## Super QuickStart
To try k0s out on Linux (we recommend Amazon Linux 2 for quick experiments: use a t2.medium or larger VM for now), run the install script at (http://) *k0s.sh*, which identifies processor type, grabs the latest compatible k0s binary, saves it as _k0s_, and makes it executable.

```
sudo curl -sSLf k0s.sh | sh
```

Then run k0s with the server option to create a k0s control plane, using the `--enable-worker` argument, which also starts and joins a worker node on the same machine.

```
sudo k0s server --enable-worker
```
Watch k0s start up for a few seconds (if issues, see the [troubleshooting guide](https://github.com/k0sproject/k0s/blob/main/docs/troubleshooting.md)), then open another terminal window (leaving k0s to percolate). In the new window, start by installing and validating kubectl:

```
curl -LO "https://storage.googleapis.com/kubernetes-release/release/$(curl -s https://storage.googleapis.com/kubernetes-release/release/stable.txt)/bin/linux/amd64/kubectl"
chmod +x kubectl
sudo mv ./kubectl /usr/local/bin/kubectl
kubectl version
```
Then retrieve the kubeconfig k0s has created for you in `/var/lib/k0s/pki/admin.conf`, and set up a shell variable that will let kubectl find and use it:

```
sudo cp /var/lib/k0s/pki/admin.conf ~/admin.conf
export KUBECONFIG=~/admin.conf
```
You should then be able to access k0s locally with kubectl commands:

```
$ kubectl get namespaces
default           Active   3m
kube-node-lease   Active   3m
kube-public       Active   3m
kube-system       Active   3m

```
<a name="better_quickstart"></a>
## Better QuickStart
The following .gif demo shows a more-convenient scenario, where the creator begins by asking k0s to generate a local k0s.yaml configuration file:

```
sudo k0s default-config > k0s.yaml
```
Then they add the public IP address of their AWS Linux 2 server to the `sans:` list, ensuring that k0s will stand up able to accept connections on this IP (as opposed to just localhost).

If you do the same thing (and remember to open port 6443 on your AWS security group for the instance) you can then install [Lens, the Kubernetes IDE](https://k8slens.dev/) on your laptop, retrieve the kubeconfig in `/var/lib/k0s/pki/admin.conf`, and copy it to a new cluster definition. Before initializing the connection, replace `localhost` in the config with the IP or FQDN of your instance. Clicking "Add Cluster" will then connect Lens to your new k0s cluster, letting you explore and interact with it freely.  

<a name="nanodemo"></a>
## k0s NanoDemo

![k0s demo](k0s_demo.gif)
