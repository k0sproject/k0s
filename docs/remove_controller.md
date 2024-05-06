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

Delete Autopilot's `ControlNode` object for the controller node:

```console
k0s kubectl delete controlnode.autopilot.k0sproject.io <controller>
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

### Declarative Etcd member management

Starting from version 1.30, k0s also supports a declarative way to remove an  
etcd member. Since in k0s the etcd cluster is set up so that the etcd API is  
**NOT** exposed outside the nodes, it makes it difficult for external automation  
like Cluster API, Terraform, etc. to handle controller node replacements.

Each controller manages their own `EtcdMember` object.

```shell
k0s kubectl get etcdmember
NAME          PEER ADDRESS   MEMBER ID           JOINED   RECONCILE STATUS
controller0   172.17.0.2     b8e14bda2255bc24    True     
controller1   172.17.0.3     cb242476916c8a58    True     
controller2   172.17.0.4     9c90504b1bc867bb    True 
```

By marking an `EtcdMember` object to leave the etcd cluster, k0s will handle the  
interaction with etcd. For example, in a 3 controller HA setup, you can  
remove a member by flagging it to leave:

```console
$ kubectl patch etcdmember controller2 -p '{"spec":{"leave":true}}' --type merge
etcdmember.etcd.k0sproject.io/controller2 patched
```

The join/leave status is tracked in the object's conditions. This allows you to  
wait for the leave to actually happen:

```console
$ kubectl wait etcdmember controller2 --for condition=Joined=False
etcdmember.etcd.k0sproject.io/controller2 condition met
```

You'll see the node left etcd cluster:

```console
$ k0s kc get etcdmember
NAME          PEER ADDRESS   MEMBER ID           JOINED   RECONCILE STATUS
controller0   172.17.0.2     b8e14bda2255bc24    True     
controller1   172.17.0.3     cb242476916c8a58    True     
controller2   172.17.0.4     9c90504b1bc867bb    False    Success
```

```console
$ k0s etcd member-list
{"members":{"controller0":"https://172.17.0.2:2380","controller1":"https://172.17.0.3:2380"}}
```

The objects for members that have already left the etcd cluster are kept  
available for tracking purposes. Once the member has left the cluster, the  
object status will reflect that it is safe to remove it.

**Note:** If you re-join same node without removing the corresponding `etcdmember` object the desired state will be updated back to `spec.leave: false` automatically. This is since currently in k0s there's no easy way to prevent a node joining etcd cluster.

## Replace a controller

To replace a controller, you first remove the old controller (like described above) then follow the [manual installation procedure](k0s-multi-node.md) to add the new one.
