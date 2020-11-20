# k0s Documentation and Resources

We encourage everyone to try k0s, and join the community. Here are some links to get you started:

## Tutorials

- **Super quick start** - Run a k0s control plane and worker node on your (Linux) laptop (or almost any medium-sized Linux VM) with one command. Access with Lens and explore.
- [**Create a k0s cluster**](https://github.com/k0sproject/k0s/blob/main/docs/create-cluster.md) - Basic walkthrough of how to build a slightly bigger k0s cluster.

## k0s Documentation

- [**k0s CLI commands**](https://github.com/k0sproject/k0s/tree/main/docs/cli) - Commands for building k0s clusters.
- [**k0s config files**](https://github.com/k0sproject/k0s/blob/main/docs/configuration.md) - Adapt and customize k0s for your use-case.
- [**k0s in Docker**](https://github.com/k0sproject/k0s/blob/main/docs/k0s-in-docker.md) - How to run k0s inside Docker (for example, to create an isolated, self-restarting k0s cluster on your laptop for development work).
- [**k0s manifests**](https://github.com/k0sproject/k0s/blob/main/docs/manifests.md) - Let k0s deploy your favorite services and apps.
- [**k0s CNI networking**](https://github.com/k0sproject/k0s/blob/main/docs/network.md) - How Calico works with k0s.
- [**k0s containerd**](https://github.com/k0sproject/k0s/blob/main/docs/containerd_config.md) - Configuring containerd with k0s.
- [**BYO CRI**](https://github.com/k0sproject/k0s/blob/main/docs/custom-cri-runtime.md) - Run your favorite container engine with k0s.
- [**Cloud providers**](https://github.com/k0sproject/k0s/blob/main/docs/cloud-providers.md) - Enabling cloud providers to work with k0s.
- [**k0s troubleshooting**](https://github.com/k0sproject/k0s/blob/main/docs/troubleshooting.md) - Sometimes, things don't work right. Here are some simple fixes when that happens.
- [**Building k0s**](https://github.com/k0sproject/k0s/blob/main/docs/building_k0s_from_source.md) - Several ways to build k0s.

## k0s Architecture
- [**k0s architecture**](https://github.com/k0sproject/k0s/blob/main/docs/architecture.md) - How the bits fit together.
- [**k0s controller processes**](https://github.com/k0sproject/k0s/blob/main/docs/k0s_controller_processes.png) - Layout of the k0s control plane.
- [**k0s worker nodes**](https://github.com/k0sproject/k0s/blob/main/docs/k0s_worker_processes.png) - Anatomy of k0s worker nodes.
- [**k0s packaging**](https://github.com/k0sproject/k0s/blob/main/docs/k0s_packaging.png) - How k0s binaries are made ("when two goroutines really love each other ...").
- [**k0s conformance testing**](https://github.com/k0sproject/k0s/blob/main/docs/conformance-testing.md) - Testing k0s to make sure it conforms to upstream Kubernetes.
