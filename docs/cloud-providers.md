# Cloud providers

k0s builds Kubernetes components in *providerless* mode, meaning that cloud providers are not built into k0s-managed Kubernetes components. As such, you must externally configure the cloud providers to enable their support in your k0s cluster (for more information on running Kubernetes with cloud providers, refer to the [Kubernetes documentation](https://kubernetes.io/docs/tasks/administer-cluster/running-cloud-controller/).

## External Cloud Providers

### Enable cloud provider support in kubelet

Even when all components are built with providerless mode, you must be able to enable cloud provider mode for kubelet. To do this, run the workers with `--enable-cloud-provider=true`.

When deploying with [k0sctl](k0sctl-install.md), you can add this into the `installFlags` of worker hosts.

```yaml
spec:
  hosts:
  - ssh:
      address: 10.0.0.1
      user: root
      keyPath: ~/.ssh/id_rsa
    installFlags:
      - --enable-cloud-provider
      - --kubelet-extra-args="--cloud-provider=external"
    role: worker
```

### Deploy the cloud provider

The easiest way to deploy cloud provider controllers is on the k0s cluster.

Use the built-in [manifest deployer](manifests.md) built into k0s to deploy your cloud provider as a k0s-managed stack. Next, just drop all required manifests into the `/var/lib/k0s/manifests/aws/` directory, and k0s will handle the deployment.

**Note**: The prerequisites for the various cloud providers can vary (for example, several require that configuration files be present on all of the nodes). Refer to your chosen cloud provider's documentation as necessary.

## k0s Cloud Provider

Alternatively, k0s provides its own lightweight cloud provider that can be used to statically assign `ExternalIP` values to worker nodes via Kubernetes annotations.  This is beneficial for those who need to expose worker nodes externally via static IP assignments.

To enable this functionality, add the parameter `--enable-k0s-cloud-provider=true` to all controllers, and `--enable-cloud-provider=true` to all workers.

Adding a static IP address to a node using `kubectl`:

```shell
kubectl annotate \
    node <node> \
    k0sproject.io/node-ip-external=<external IP>
```

Both IPv4 and IPv6 addresses are supported.

### Defaults

The default node refresh interval is `2m`, which can be overridden using the `--k0s-cloud-provider-update-frequency=<duration>` parameter when launching the controller(s).

The default port that the cloud provider binds to can be overridden using the `--k0s-cloud-provider-port=<int>` parameter when launching the controller(s).
