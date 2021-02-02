# Dual-stack networking

To enable dual-stack networking use the following k0s.yaml as an example.
This settings will set up bundled calico cni, enable feature gates for the Kubernetes components and set up kubernetes-controller-manager.
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
## CNI settings

### Calico settings

Calico doesn't support tunneling for the IPv6, so "vxlan" and "ipip" backend wouldn't work. 
If you need to have cross-pod connectivity, you need to use "bird" as a backend mode. 
In any other mode the pods would be able to reach only pods on the same node.

### External CNI
The `k0s.yaml` dualStack section will enable all the neccessary feature gates for the Kubernetes components but in case of using external CNI it must be set up with IPv6 support.
 
## Additional materials
https://kubernetes.io/docs/concepts/services-networking/dual-stack/

https://kubernetes.io/docs/tasks/network/validate-dual-stack/ 

https://www.projectcalico.org/dual-stack-operation-with-calico-on-kubernetes/

https://docs.projectcalico.org/networking/ipv6