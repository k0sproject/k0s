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
worker0   NotReady   <none>   10s   v{{{ extra.k8s_version }}}+k0s  beta.kubernetes.io/arch=amd64,beta.kubernetes.io/os=linux,k0sproject.io/foo=bar,k0sproject.io/other=xyz,kubernetes.io/arch=amd64,kubernetes.io/hostname=worker0,kubernetes.io/os=linux
```

Controller worker nodes are assigned `node.k0sproject.io/role=control-plane` and `node-role.kubernetes.io/control-plane=true` labels:

```shell
kubectl get node --show-labels
```

```shell
NAME          STATUS     ROLES           AGE   VERSION        LABELS
controller0   NotReady   control-plane   10s   v{{{ extra.k8s_version }}}+k0s  beta.kubernetes.io/arch=amd64,beta.kubernetes.io/os=linux,kubernetes.io/hostname=worker0,kubernetes.io/os=linux,node.k0sproject.io/role=control-plane,node-role.kubernetes.io/control-plane=true
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

## Kubelet configuration

The `k0s worker` command accepts a generic flag to pass in any set of arguments
for kubelet process.

For example, running `k0s worker --token-file=k0s.token
--kubelet-extra-args="--node-ip=1.2.3.4 --address=0.0.0.0"` passes in the given
flags to Kubelet as-is. As such, you must confirm that any flags you are passing
in are properly formatted and valued as k0s will not validate those flags.

### Worker Profiles

Kubelet configuration fields can also be set via a worker profiles. Worker
profiles are defined in the main k0s.yaml and are used to generate ConfigMaps
containing a custom `kubelet.config.k8s.io/v1beta1/KubeletConfiguration` object.
To see examples of k0s.yaml containing worker profiles: [go
here](./configuration.md#specworkerprofiles). For a list of possible Kubelet
configuration fields: [go
here](https://kubernetes.io/docs/reference/config-api/kubelet-config.v1beta1/).

## IPTables Mode

k0s detects iptables backend automatically based on the existing records. On a brand-new setup, `iptables-nft` will be used.  
There is a `--iptables-mode` flag to specify the mode explicitly. Valid values: `nft`, `legacy` and `auto` (default).

```shell
k0s worker --iptables-mode=nft
```
