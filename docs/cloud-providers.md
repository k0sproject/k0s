# Cloud providers

K0s supports all [Kubernetes cloud controllers]. However, those must be installed as separate cluster add-ons since k0s builds Kubernetes components in *providerless* mode.

[Kubernetes cloud controllers]: https://kubernetes.io/docs/concepts/architecture/cloud-controller/

## Enable cloud provider support in kubelet

You must enable cloud provider mode for kubelet. To do this, run the workers with `--enable-cloud-provider=true`.

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

## Deploy the cloud provider

You can use any means to deploy your cloud controller into the cluster. Most providers support [Helm charts](helm-charts.md) to deploy them.

**Note**: The prerequisites for the various cloud providers can vary (for example, several require that configuration files be present on all of the nodes). Refer to your chosen cloud provider's documentation as necessary.

## k0s Cloud Provider

Alternatively, k0s provides its own lightweight cloud provider that can be used to statically assign `ExternalIP` values to worker nodes via Kubernetes annotations.  This is beneficial for those who need to expose worker nodes externally via static IP assignments.

To enable this functionality, add the parameter `--enable-k0s-cloud-provider=true` to all controllers, and `--enable-cloud-provider=true` to all workers.

Adding a static IP address to a node using `kubectl`:

```shell
kubectl annotate \
    node <node> \
    k0sproject.io/node-ip-external=<external IP>[,<external IP 2>][,<external IP 3>]
```

Both IPv4 and IPv6 addresses and multiple comma-separated values are supported.

### Defaults

The default node refresh interval is `2m`, which can be overridden using the `--k0s-cloud-provider-update-frequency=<duration>` parameter when launching the controller(s).

The default port that the cloud provider binds to can be overridden using the `--k0s-cloud-provider-port=<int>` parameter when launching the controller(s).
