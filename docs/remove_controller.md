# Remove or replace a controller

You can manually remove or replace a controller from a multi-node k0s cluster (>=3 controllers) without downtime.

## Remove a controller

You first have to delete the controller from Kubernetes itself.
To do so, run the following commands from the controller:

```shell
k0s kubectl drain --ignore-daemonsets --delete-emptydir-data <controller>
k0s kubectl delete node <controller>
```

Then you need to remove it from the etcd cluster.

```shell
PEER_ADRESS=$(k0s etcd member-list | python -m json.tool | grep <controller> | cut -d"\"" -f4)
k0s etcd leave --peer-address $PEER_ADRESS
```

The controller is now removed from the cluster.
To [reset k0s on the machine](reset.md), run the following commands:

```shell
k0s reset
reboot
```

## Replace a controller

To replace a controller, you first remove the old controller (like described above) then follow the [Manual install procedure](k0s-multi-node.md) to add the new one.
