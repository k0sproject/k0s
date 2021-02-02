# Dual-stack networking

At the moment we support dual-stack networking only with calico CNI. 

To enable dual-stack networking use the following k0s.yaml as an example:

```
spec:
  network:
    podCIDR: "10.244.0.0/16"
    serviceCIDR: "10.96.0.0/12"
    calico:
      mode: "bird"
    dualStack:
      enabled: true
      IPv6podCIDR: "fd00::/108"
      IPv6serviceCIDR: "fd01::/108"
```

## Calico settings

Calico doesn't support tunneling for the IPv6, so "vxlan" and "ipip" backend wouldn't work. 
If you need to have cross-pod connectivity, you need to use "bird" as a backend mode. 
In any other mode the pods would be able to reach only pods on the same node.


### Additional materials
https://kubernetes.io/docs/concepts/services-networking/dual-stack/

https://kubernetes.io/docs/tasks/network/validate-dual-stack/ 

https://www.projectcalico.org/dual-stack-operation-with-calico-on-kubernetes/

https://docs.projectcalico.org/networking/ipv6