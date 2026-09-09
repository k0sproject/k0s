<!--
SPDX-FileCopyrightText: 2021 k0s authors
SPDX-License-Identifier: CC-BY-SA-4.0
-->

# Configuration options for worker nodes

Although the `k0s worker` command does not take in any special YAML configuration, there are still methods for configuring the workers to run various components.

## Node labels

The `k0s worker` command accepts the `--labels` flag, with which you can make the newly joined worker node register itself, in the Kubernetes API, with the given set of labels.

For example, running the worker with `k0s worker --token-file k0s.token --labels="k0sproject.io/foo=bar,k0sproject.io/other=xyz"` results in:

{% set kubelet_ver = k8s_version + '+k0s' -%}
{% set kubelet_ver_len = kubelet_ver | length -%}

```console
$ kubectl get node --show-labels
NAME      STATUS     ROLES    AGE   {{{ 'VERSION'   | ljust(kubelet_ver_len) }}}   LABELS
worker0   NotReady   <none>   10s   {{{ kubelet_ver | ljust(kubelet_ver_len) }}}   beta.kubernetes.io/arch=amd64,beta.kubernetes.io/os=linux,k0sproject.io/foo=bar,k0sproject.io/other=xyz,kubernetes.io/arch=amd64,kubernetes.io/hostname=worker0,kubernetes.io/os=linux
```

Controller worker nodes are assigned `node.k0sproject.io/role=control-plane` and `node-role.kubernetes.io/control-plane=true` labels:

```console
$ kubectl get node --show-labels
NAME          STATUS     ROLES           AGE   {{{ 'VERSION'   | ljust(kubelet_ver_len) }}}   LABELS
controller0   NotReady   control-plane   10s   {{{ kubelet_ver | ljust(kubelet_ver_len) }}}   beta.kubernetes.io/arch=amd64,beta.kubernetes.io/os=linux,kubernetes.io/hostname=worker0,kubernetes.io/os=linux,node.k0sproject.io/role=control-plane,node-role.kubernetes.io/control-plane=true
```

**Note:** Setting the labels is only effective on the first registration of the node. Changing the labels thereafter has no effect.

## Taints

The `k0s worker` command accepts the `--taints` flag, with which you can make the newly joined worker node register itself with the given set of taints.

**Note:** Controller nodes running with `--enable-worker` are assigned `node-role.kubernetes.io/control-plane:NoExecute` taint automatically. You can disable default taints using `--no-taints` parameter.

```shell
kubectl get nodes -o custom-columns=NAME:.metadata.name,TAINTS:.spec.taints
```

```shell
NAME          TAINTS
controller0   [map[effect:NoSchedule key:node-role.kubernetes.io/control-plane]]
worker0       <none>
```

## Kubelet configuration

The `k0s worker` command accepts a generic flag to pass in any set of arguments
for the kubelet process.

For example, running `k0s worker --token-file=k0s.token
--kubelet-extra-args="--node-ip=1.2.3.4 --address=0.0.0.0"` passes in the given
flags to Kubelet as-is. As such, you must confirm that any flags you are passing
in are properly formatted and valued as k0s will not validate those flags.

### Worker Profiles

Kubelet configuration fields can also be set via worker profiles. Worker
profiles are defined in the main k0s.yaml and are used to generate ConfigMaps
containing a custom `kubelet.config.k8s.io/v1beta1/KubeletConfiguration`
object.
See also the [examples of k0s.yaml containing worker
profiles](./configuration.md#configuration-examples) and the [list of possible
Kubelet configuration
fields](https://kubernetes.io/docs/reference/config-api/kubelet-config.v1beta1/).

### Kubelet serving certificates

By default, k0s configures kubelets to request their serving certificates from
the Kubernetes API via [Certificate Signing Requests] (CSRs), by enabling
`serverTLSBootstrap` in the generated kubelet configuration. The kubelet then
requests a certificate from the `kubernetes.io/kubelet-serving` signer and uses
it to serve its own API. Clients of that API verify the certificate against the
cluster CA. The Kubernetes API server verifies it, for example, when serving
`kubectl logs` and `kubectl exec` requests. Similarly, k0s's `metrics-server`
component, which bundles the [Kubernetes Metrics Server], verifies it when
scraping the kubelet's resource metrics. Other properly configured monitoring
systems do the same.

Kubernetes doesn't approve these CSRs automatically. k0s ships a controller
component called `csr-approver` that does so, provided the request meets all of
the following conditions:

- It is a well-formed kubelet serving certificate request, according to
  Kubernetes' validation rules.
- It was created by the very node it requests the certificate for. The
  requesting user must have the `system:node:<nodeName>` name and be a member of
  the `system:nodes` group, and the certificate's common name must be identical
  to the requesting user name.
- The node exists in the cluster.
- The requested DNS names and IP addresses are all listed in the node's
  `status.addresses`.

Requests that don't meet these conditions are left pending, and the reason is
logged by the k0s controller. Note that a node's addresses are reported by its
kubelet, so flags like `--node-ip` directly influence which addresses a
certificate may be issued for.

The CSR approver can be turned off via `k0s controller --disable-components
csr-approver`. In that case, kubelet serving certificates need to be approved by
some other means, otherwise the affected kubelet APIs remain unavailable.

[Certificate Signing Requests]: https://kubernetes.io/docs/reference/access-authn-authz/certificate-signing-requests/
[Kubernetes Metrics Server]: https://github.com/kubernetes-sigs/metrics-server

## IPTables Mode

k0s detects the iptables backend automatically based on the existing records. On a brand-new setup, `iptables-nft` will be used.
There is an `--iptables-mode` flag to specify the mode explicitly. Valid values: `nft`, `legacy` and `auto` (default).

```shell
k0s worker --iptables-mode=nft
```
