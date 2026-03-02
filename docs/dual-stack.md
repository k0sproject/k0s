<!--
SPDX-FileCopyrightText: 2021 k0s authors
SPDX-License-Identifier: CC-BY-SA-4.0
-->

# IPv4/IPv6 dual-stack networking

Enabling dual-stack networking in k0s allows your cluster to handle both IPv4 and
IPv6 addresses. Follow the configuration examples below to set up dual-stack mode.

## Enabling dual-stack using the default CNI (Kube-router)

In order to enable dual-stack networking using the default CNI provider, use the
following example configuration:

```yaml
spec:
  network:
    # Kube-router is the default CNI provider
    # provider: kube-router
    podCIDR: 10.244.0.0/16
    serviceCIDR: 10.96.0.0/12
    dualStack:
      enabled: true
      IPv6podCIDR: fd00::/108
      IPv6serviceCIDR: fd01::/108
```

This configuration will set up all Kubernetes components and Kube-router
accordingly for dual-stack networking.

### How Auto-detection Works

By default, k0s attempts to auto-detect the node's IPv4 and IPv6 addresses to facilitate dual-stack networking. This is necessary because the upstream Kubelet does not natively support node IP auto-detection when running in dual-stack mode.

If you do not manually specify node IPs, k0s replicates the Kubelet's logic by:

- **DNS Lookup:** Attempting to resolve the node's hostname via the system DNS resolver to find associated IPv4 and IPv6 addresses.
- **Interface Scanning:** Looking up IPs directly via the local network interface used as a default gateway.

> [!IMPORTANT]
> For the DNS Lookup mechanism to work reliably, your system resolver must be able to return both address families for the node hostname.

---

> [!NOTE]
> You can bypass the auto-detection mechanism entirely—and avoid the DNS requirement—by explicitly defining the node IPs.
> Add the `--node-ip` flag to your [Kubelet extra arguments](https://docs.k0sproject.io/stable/worker-node-config/#kubelet-configuration):
>
> ```bash
> --node-ip=<IPv4_ADDRESS>,<IPv6_ADDRESS>
> ```

### Configuring the node CIDR mask size

By default, k0s uses a `/117` node CIDR mask size for IPv6, which provides 2048
IP addresses per node and a `/24` for IPv4 which provides 256 addresses per node.

For IPv6, using the example configuration `IPv6PodCIDR: fd00::/108`, there
are 9 bits available for node allocation (117 - 108 = 9) and 11 bits available for
pod allocation (128 - 117 = 11). This allows for 512 nodes per cluster and 2048
IPs per node.

For IPv4, using the default `PodCIDR: 10.244.0.0/16`, there are 8 bits available
for node allocation and 8 bits available for pod allocation. This allows for 256
nodes per cluster and 256 IPs per node.
per cluster and 256 IPs per node.

You can customize the node CIDR mask size using the controller manager's extra arguments:

```yaml
spec:
  controllerManager:
    extraArgs:
      node-cidr-mask-size-ipv6: "120"
      node-cidr-mask-size-ipv4: "21"
  network:
    dualStack:
      enabled: true
      IPv6podCIDR: fd00::/108
      IPv6serviceCIDR: fd01::/108
```

## Using Calico as the CNI provider

Calico does not support IPv6 tunneling in the default `vxlan` mode, so if you
prefer to use Calico as your CNI provider, make sure to select `bird` mode. Use
the following example configuration:

```yaml
spec:
  network:
    provider: calico
    calico:
      mode: bird
    podCIDR: 10.244.0.0/16
    serviceCIDR: 10.96.0.0/12
    dualStack:
      enabled: true
      IPv6podCIDR: fd00::/108
      IPv6serviceCIDR: fd01::/108
```

## Specifying the default IP family

In Kubernetes dual stack clusters, by default all the services are single stack,
including `kubernetes.default.svc`, which is used to communicate with the
Kubernetes API servers.

This is specially important when specifying explicitly
`spec.api.externalAddress` or `spec.api.address`.

To explicitly define the family which will be used by default use the following
configuration:

```yaml
spec:
  network:
    # primaryAddressFamily is optional
    primaryAddressFamily: <IPv4|IPv6>
```

If not defined explicitly, k0s will determine it based on `spec.api.externalAddress`,
if this field is not defined, k0s will use `spec.api.address`. If the field used is
a host name or both are empty, k0s will use IPv4.

## Custom CNI providers

While the dual-stack configuration section configures all components managed by
k0s for dual-stack operation, the custom CNI provider must also be configured
accordingly. Refer to the documentation for your specific CNI provider to ensure
a proper dual-stack setup that matches that of k0s.

## Additional Resources

For more detailed information and troubleshooting, refer to the following resources:

- <https://kubernetes.io/docs/concepts/services-networking/dual-stack/>
- <https://kubernetes.io/docs/tasks/network/validate-dual-stack/>
- <https://www.tigera.io/blog/dual-stack-operation-with-calico-on-kubernetes/>
- <https://docs.tigera.io/calico/3.27/networking/ipam/ipv6#enable-dual-stack>
