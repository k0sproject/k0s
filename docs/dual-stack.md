# Dual-stack Networking

**Note:** Dual stack networking setup requires that you configure Calico or a custom CNI as the CNI provider.

Use the following `k0s.yaml` as a template to enable dual-stack networking. This configuration will set up bundled calico CNI, enable feature gates for the Kubernetes components, and set up `kubernetes-controller-manager`.

```yaml
spec:
  network:
    podCIDR: "10.244.0.0/16"
    serviceCIDR: "10.96.0.0/12"
    provider: calico
    calico:
      mode: "bird"
    dualStack:
      enabled: true
      IPv6podCIDR: "fd00::/108"
      IPv6serviceCIDR: "fd01::/108"
```

## CNI Settings: Calico

For cross-pod connectivity, use BIRD for the backend. Calico does not support tunneling for the IPv6, and thus VXLAN and IPIP backends do not work.

**Note**: In any Calico mode other than cross-pod, the pods can only reach pods on the same node.

## CNI Settings: External CNI

Although the `k0s.yaml` dualStack section enables all of the neccessary feature gates for the Kubernetes components, for use with an external CNI it must be set up to support IPv6.

## Additional Resources

* https://kubernetes.io/docs/concepts/services-networking/dual-stack/
* https://kubernetes.io/docs/tasks/network/validate-dual-stack/
* https://www.projectcalico.org/dual-stack-operation-with-calico-on-kubernetes/
* https://docs.projectcalico.org/networking/ipv6