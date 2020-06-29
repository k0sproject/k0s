# MKE - Mirantis Kubernetes Engine


## Build

```
$ make build
```


## Cluster bootstrapping

Currently mke makes only very basic default (hardcoded) configs for everything.

Move the built `mke` binary to eahc of the nodes.

### Control plane

Currently only single node control planes are supported!

`mke server`

This create all the necessary certs and configs in `/var/lib/mke/pki`. Mke runs all control plane components in separate "naked" processes, does not depend on kubelet or container engine.

After control plane boots up, we need to create a join token for worker node:
```
mke token create
```

*Note:* The token is super long atm, we intend to make it shorter at some point

### Worker node

Join a new worker node to the cluster by running:
```
mke worker --join-token "superlongtokenfrompreviousphase"
```