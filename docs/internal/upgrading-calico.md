<!--
SPDX-FileCopyrightText: 2020 k0s authors
SPDX-License-Identifier: CC-BY-SA-4.0
-->

# Upgrading Calico

K0s bundles Kubernetes manifests for Calico. The manifests are retrieved from
the [official Calico repo].

As fetching and modifying the entire multi-thousand line file is error-prone,
you may follow these steps to upgrade Calico to the latest version:

1. run `./hack/get-calico.sh <version>`
2. check the git diff to see if it looks sensible
3. re-apply our manual adjustments (documented below)
4. compile, pray, and test
5. commit and create a PR

[official Calico repo]: https://github.com/projectcalico/calico/blob/master/manifests/calico.yaml

## Manual Adjustments

All manual adjustments should be visible in the git diff. This section provides
a checklist to ensure that these changes are still applied correctly. The code
blocks in this section are k0s-specific modifications. Any such modification
should be accompanied by Go template comments to distinguish them from the
original code.

To see the diff **without** CRDs, you can do something like:

```sh
git diff ':!static/manifests/calico/CustomResourceDefinition'
```

That'll make it easier to spot any needed changes.

`static/manifests/calico/DaemonSet/calico-node.yaml`:

### Inlining the `calico-config` ConfigMap

Instead of using a ConfigMap to manage the Calico backend and MTU settings,
incorporate them directly into the places where they're used. This will trigger
rolling updates whenever something changes.

This includes the template for the node's CNI plugin configuration (the
`cni_network_config` key in the ConfigMap, the `CNI_NETWORK_CONFIG` environment
variable of the init container for the `calico-node` DaemonSet). Calico's CNI
network plugin installer executable ships with a default template, but k0s needs
to tweak this a bit for IPv6 support.

### Support for switching Calico backends

Search for `.Mode` to find:

```yaml
# Enable IPIP
- name: CALICO_IPV4POOL_IPIP
  value: "{{ if eq .Mode "bird" }}{{ .Overlay }}{{ else }}Never{{ end }}"
# Enable or Disable VXLAN on the default IP pool.
- name: CALICO_IPV4POOL_VXLAN
  value: "{{ if and (eq .Mode "vxlan") .EnableIPv4 }}{{ .Overlay }}{{ else }}Never{{ end }}"
# Enable or Disable VXLAN on the default IPv6 IP pool.
- name: CALICO_IPV6POOL_VXLAN
  value: "{{ if and (eq .Mode "vxlan") .EnableIPv6 }}{{ .Overlay }}{{ else }}Never{{ end }}"
{{- if eq .Mode "vxlan" }}
- name: FELIX_VXLANPORT
  value: "{{ .VxlanPort }}"
- name: FELIX_VXLANVNI
  value: "{{ .VxlanVNI }}"
{{- end }}
```

### iptables auto-detection

```yaml
# Auto detect the iptables backend
- name: FELIX_IPTABLESBACKEND
  value: "auto"
```

### Support for enabling WireGuard

```yaml
{{- if .EnableWireguard }}
- name: FELIX_WIREGUARDENABLED
  value: "true"
{{- end }}
```

### Support for changing the cluster CIDR

```yaml
- name: CALICO_IPV4POOL_CIDR
  value: "{{ .ClusterCIDR }}"
```

### Support for changing the MTU

Search for `.MTU` to find:

```yaml
# Set MTU for tunnel device used if ipip is enabled
- name: FELIX_IPINIPMTU
  value: "{{ .MTU }}"
# Set MTU for the VXLAN tunnel device.
- name: FELIX_VXLANMTU
  value: "{{ .MTU }}"
# Set MTU for the Wireguard tunnel device.
- name: FELIX_WIREGUARDMTU
  value: "{{ .MTU }}"
```

### Remove bgp support

Remove `bgp` from `CLUSTER_TYPE`:

```yaml
- name: CLUSTER_TYPE
  value: "k8s"
```

Disable BIRD checks on liveness and readiness probes when not running the BIRD
backend:

```yaml
{{- if eq .Mode "bird" }}
- -bird-live
{{- end }}
```

```yaml
{{- if eq .Mode "bird" }}
- -bird-ready
{{- end }}
```

### Remove eBPF support

Comment out the parts of the manifests related to eBPF support. They are usually
commented as such by upstream.

### Disable VXLAN offloading

See <https://github.com/k0sproject/k0s/issues/3024> for details.

```yaml
- name: FELIX_FEATUREDETECTOVERRIDE
  value: ChecksumOffloadBroken=true
```

### Template container image names

Instead of hardcoded image names and versions use placeholders to support
configuration level settings. Following placeholders are used:

- `CalicoCNIImage` for calico/cni
- `CalicoNodeImage` for calico/node
- `CalicoKubeControllersImage` for calico/kube-controllers

Also, all containers in manifests were modified to have the `imagePullPolicy`
field:

```yaml
imagePullPolicy: {{ .PullPolicy }}
```
