<!--
SPDX-FileCopyrightText: 2025 k0s authors
SPDX-License-Identifier: CC-BY-SA-4.0
-->

# IPv6 single-stack networking

K0s introduced experimental IPv6 single-stack networking, allowing clusters to operate
exclusively with IPv6 addresses. IPv6 single stack is currently in alpha state and requires
the [IPv6SingleStack feature gate](feature-gates.md) to be enabled on every controller.

IPv6 single-stack is fully compatible with both [node-local load balancing](nllb.md) and
[control plane load balancing](cplb.md).

## Enabling IPv6 single-stack using the default CNI (Kube-router)

In order to enable IPv6 single-stack networking using the default CNI provider, both an IPv6
`podCIDR` and `serviceCIDR` must be provided. Dual stack must not be enabled. Migrations
to dual stack are possible as long as the primary address family remains IPv6.

```yaml
spec:
  network:
    podCIDR: fd00::/108
    serviceCIDR: fd01::/108
```

Ensure that every controller in the cluster is executed with the
[IPv6SingleStack feature gate](feature-gates.md) enabled:

```shell
k0s controller --feature-gates="IPv6SingleStack=true"
```

This configuration will set up all Kubernetes components and Kube-router for IPv6
single-stack networking.

### Configuring the node CIDR mask size

By default, k0s uses a `/117` node CIDR mask size for IPv6, which provides
2048 IP addresses per node and and a `/24` for IPv4 which provides 256
addresses per node.

Using the example configuration `IPv6PodCIDR: fd00::/108`, there are 9 bits
available for node allocation (117 - 108 = 9) and 11 bits available for
pod allocation (128 - 117 = 11). This allows 512 nodes with 2048 IPs per node.

You can customize the node CIDR mask size using the controller manager's extra arguments:

```yaml
spec:
  controllerManager:
    extraArgs:
      node-cidr-mask-size: "120"
  network:
    podCIDR: fd00::/108
    serviceCIDR: fd01::/108
```

With a `/120` node CIDR mask size, the cluster would support more nodes (4096 nodes maximum
with the `/108` pod CIDR) but each node would receive 256 IP addresses.

### Important Notes for Kube-router

Currently kube-router on IPv6 doesn't pass the kubernetes network conformance tests.
