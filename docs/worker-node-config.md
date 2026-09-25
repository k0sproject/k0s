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

- The PEM-encoded certificate request size doesn't exceed 512 KiB.
- It is a well-formed kubelet serving certificate request, according to
  Kubernetes' validation rules.
- The certificate request has no more than 4096 SANs, counting DNS names and IP
  addresses together.
- It was created by the very node it requests the certificate for. The
  requesting user must have the `system:node:<nodeName>` name and be a member of
  the `system:nodes` group, and the certificate's common name must be identical
  to the requesting user name.
- The requested DNS names and IP addresses don't identify the cluster's control
  plane or other in-cluster services: no IP address may be inside the service
  CIDR(s), and no DNS name may be `kubernetes`, `kubernetes.default`, or a name
  within the `svc` or cluster domains.
- The node exists in the cluster.
- The node's `status.addresses` contains no more than 4096 entries.
- The requested DNS names and IP addresses are all listed in the node's
  `status.addresses`.

Requests that don't meet these conditions are left pending, and the reason is
logged by the k0s controller. Note that a node's addresses are reported by its
kubelet, so flags like `--node-ip` directly influence which addresses a
certificate may be issued for.

The CSR approver checks for pending requests periodically. It only considers the
newest pending request of each node per pass, so that a node can't hold up
others by creating large numbers of requests. It can be turned off via `k0s
controller --disable-components csr-approver`. In that case, kubelet serving
certificates need to be approved by some other means, otherwise the affected
kubelet APIs remain unavailable.

[Certificate Signing Requests]: https://kubernetes.io/docs/reference/access-authn-authz/certificate-signing-requests/
[Kubernetes Metrics Server]: https://github.com/kubernetes-sigs/metrics-server

## Starting with a cached worker profile

A worker loads its worker profile from the Kubernetes API when it starts. If the
API cannot be reached, the worker retries for a few minutes and then exits, so
the kubelet never comes up. On nodes that are expected to boot while the control
plane is unreachable, such as edge nodes recovering from a power loss, this
turns a temporary outage into a node that stays down.

The `k0s worker` command accepts the `--allow-cached-config` flag to opt into a
fallback. k0s still asks the Kubernetes API first, exactly as it does without
the flag, and only if that fails does it start with the worker profile that the
previous successful load stored on disk:

```shell
k0s worker --token-file k0s.token --allow-cached-config
```

The fallback is deliberately narrow:

- It is opt-in. Without the flag, an unreachable API remains a fatal error.
- It does not apply to rejected or forbidden credentials. A node whose
  credentials are invalid still needs to be rejoined into the cluster.
- It refuses a profile that was cached for another `--profile` name or for
  another Kubernetes minor version. A profile cached by a k0s version that
  didn't record the Kubernetes version yet is refused as well; it is rewritten
  in the current format on the next successful start.
- If there is no usable cached profile, k0s reports both the API error and the
  reason the cache was unusable, and exits.

A worker that started from the cache logs a warning and runs with a
configuration that may be out of date. It keeps that configuration until it is
restarted.

## IPTables Mode

k0s detects the iptables backend automatically based on the existing records. On a brand-new setup, `iptables-nft` will be used.
There is an `--iptables-mode` flag to specify the mode explicitly. Valid values: `nft`, `legacy` and `auto` (default).

```shell
k0s worker --iptables-mode=nft
```
