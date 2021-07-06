# Cloud providers

k0s builds Kubernetes components in *providerless* mode, meaning that cloud providers are not built into k0s-managed Kubernetes components. As such, you must externally configure the cloud providers to enable their support in your k0s cluster (for more information on running Kubernetes with cloud providers, refer to the [Kubernetes documentation](https://kubernetes.io/docs/tasks/administer-cluster/running-cloud-controller/).

## 1. Enable cloud provider support in kubelet

Even when all components are built with providerless mode, you must be able to enable cloud provider mode for kubelet. To do this, run the workers with `--enable-cloud-provider=true`.

## 2. Deploy the cloud provider

The easiest way to deploy cloud provider controllers is on the k0s cluster.

Use the built-in [manifest deployer](manifests.md) built into k0s to deploy your cloud provider as a k0s-managed stack. Next, just drop all required manifests into the `/var/lib/k0s/manifests/aws/` directory, and k0s will handle the deployment.

**Note**: The prerequisites for the various cloud providers can vary (for example, several require that configuration files be present on all of the nodes). Refer to your chosen cloud provider's documentation as necessary.
