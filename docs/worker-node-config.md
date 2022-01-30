# Configuration options for worker nodes

Although the `k0s worker` command does not take in any special yaml configuration, there are still methods for configuring the workers to run various components.

## Node labels

The `k0s worker` command accepts the `--labels` flag, with which you can make the newly joined worker node the register itself, in the Kubernetes API, with the given set of labels.

For example, running the worker with `k0s worker --token-file k0s.token --labels="k0sproject.io/foo=bar,k0sproject.io/other=xyz"` results in:

```shell
kubectl get node --show-labels
```

```shell
NAME      STATUS     ROLES    AGE   VERSION        LABELS
worker0   NotReady   <none>   10s   v1.20.2-k0s1   beta.kubernetes.io/arch=amd64,beta.kubernetes.io/os=linux,k0sproject.io/foo=bar,k0sproject.io/other=xyz,kubernetes.io/arch=amd64,kubernetes.io/hostname=worker0,kubernetes.io/os=linux
```

Controller worker nodes are assigned `node.k0sproject.io/role=control-plane` and `node-role.kubernetes.io/control-plane=true` labels:

```shell
kubectl get node --show-labels
```

```shell
NAME          STATUS     ROLES           AGE   VERSION        LABELS
controller0   NotReady   control-plane   10s   v1.20.2-k0s1   beta.kubernetes.io/arch=amd64,beta.kubernetes.io/os=linux,kubernetes.io/hostname=worker0,kubernetes.io/os=linux,node.k0sproject.io/role=control-plane,node-role.kubernetes.io/control-plane=true
```

**Note:** Setting the labels is only effective on the first registration of the node. Changing the labels thereafter has no effect.

## Taints

The `k0s worker` command accepts the `--taints` flag, with which you can make the newly joined worker node the register itself with the given set of taints.

**Note:** Controller nodes running with `--enable-worker` are assigned `node-role.kubernetes.io/master:NoExecute` taint automatically. You can disable default taints using `--no-taints`  parameter.

```shell
kubectl get nodes -o custom-columns=NAME:.metadata.name,TAINTS:.spec.taints
```

```shell
NAME          TAINTS
controller0   [map[effect:NoSchedule key:node-role.kubernetes.io/master]]
worker0       <none>
```

## Kubelet args

The `k0s worker` command accepts a generic flag to pass in any set of arguments for kubelet process.

For example, running `k0s worker --token-file=k0s.token --kubelet-extra-args="--node-ip=1.2.3.4 --address=0.0.0.0"` passes in the given flags to kubelet as-is. As such, you must confirm that any flags you are passing in are properly formatted and valued as k0s will not validate those flags.
