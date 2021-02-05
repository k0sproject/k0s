# Frequently asked questions

### How is k0s pronounced?

kay-zero-ess

### How do I run a single node cluster?

`k0s controller --enable-worker`

### How do I connect to the cluster?

You find the config in `${DATADIR}/pki/admin.conf` (default: `/var/lib/k0s/pki/admin.conf`). Copy this file, and
change the `localhost` entry to the public ip of the controller. Use the
modified config to connect with kubectl:
```
export KUBECONFIG=/path/to/admin.conf
kubectl ...
```

### Why doesn't `kubectl get nodes` list the k0s controllers?

As a default, the control plane does not run kubelet at all, and will not
accept any workloads, so the controller will not show up on the node list in
kubectl. If you want your controller to accept workloads and run pods, you do so with:
`k0s controller --enable-worker` (recommended only as test/dev/POC environments).
