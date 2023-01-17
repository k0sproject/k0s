# Remove or replace a controller

You can manually remove or replace a controller from a multi-node k0s cluster (>=3 controllers) without downtime.
However, you have to [maintain quorum on Etcd](https://etcd.io/docs/v3.3/faq/#why-an-odd-number-of-cluster-members) while doing so.

## Remove a controller

If your controller is also a worker (`k0s controller --enable-worker`), you first have to delete the controller from Kubernetes itself.
To do so, run the following commands from the controller:

```shell
# Remove the containers from the node and cordon it
k0s kubectl drain --ignore-daemonsets --delete-emptydir-data <controller>
# Delete the node from the cluster
k0s kubectl delete node <controller>
```

Then you need to remove it from the Etcd cluster.
For example, if you want to remove `controller01` from a cluster with 3 controllers:

```shell
# First, list the Etcd members
k0s etcd member-list
{"members":{"controller01":"<PEER_ADDRESS1>", "controller02": "<PEER_ADDRESS2>", "controller03": "<PEER_ADDRESS3>"}}
# Then, remove the controller01 using its peer address
k0s etcd leave --peer-address "<PEER_ADDRESS1>"
```

The controller is now removed from the cluster.
To [reset k0s on the machine](reset.md), run the following commands:

```shell
k0s stop
k0s reset
reboot
```

## Replace a controller

To replace a controller, you first remove the old controller (like described above) then follow the [manual installation procedure](k0s-multi-node.md) to add the new one.
