# Upgrading Calico

k0s bundles Kubernetes manifests for Calico. The manifests are retrieved
from the [official Calico docs](https://docs.projectcalico.org/manifests/calico.yaml).

As fetching and modifying the entire multi-thousand line file is error-prone,
you may follow these steps to upgrade Calico to the latest version:

1. run `./get-calico.sh`
2. check the git diff to see if it looks sensible
3. re-apply our manual adjustments (documented below)
4. run `make bindata-manifests`
5. compile, pray, and test
6. commit and create a PR

## Manual Adjustments

**Note:** All manual adjustments should be fairly obvious from the git diff.
This section attempts to provide a sanity checklist to go through and make sure
we still have those changes applied. The code blocks in this section are **our modifications**,
not the calico originals.

To see the diff **without** CRDs, you can do something like:

```sh
git diff ':!static/manifests/calico/CustomResourceDefinition'
```

That'll make it easier to spot any needed changes.

`static/manifests/calico/DaemonSet/calico-node.yaml`:

- variable-based support for both vxlan and ipip (search for `ipip` to find):

```yaml
{{- if eq .Mode "ipip" }}
# Enable IPIP
- name: CALICO_IPV4POOL_IPIP
  value: {{ .Overlay }}
# Enable or Disable VXLAN on the default IP pool.
- name: CALICO_IPV4POOL_VXLAN
  value: "Never"
{{- else if eq .Mode "vxlan" }}
# Disable IPIP
- name: CALICO_IPV4POOL_IPIP
  value: "Never"
# Enable VXLAN on the default IP pool.
- name: CALICO_IPV4POOL_VXLAN
  value: {{ .Overlay }}
- name: FELIX_VXLANPORT
  value: "{{ .VxlanPort }}"
- name: FELIX_VXLANVNI
  value: "{{ .VxlanVNI }}"
{{- end }}
```

- iptables auto detect:

```yaml
# Auto detect the iptables backend
- name: FELIX_IPTABLESBACKEND
  value: "auto"
```

- variable-based WireGuard support:

```yaml
{{- if .EnableWireguard }}
- name: FELIX_WIREGUARDENABLED
  value: "true"
{{- end }}
```

- variable-based cluster CIDR:

```yaml
- name: CALICO_IPV4POOL_CIDR
  value: "{{ .ClusterCIDR }}"
```

- custom backend and MTU

```yaml
# calico-config.yaml
calico_backend: "{{ .Mode }}"
veth_mtu: "{{ .MTU }}"
```

- remove bgp from `CLUSTER_TYPE`

```yaml
- name: CLUSTER_TYPE
  value: "k8s"
```

- disable BIRD checks on liveness and readiness as we don't support BGP by removing
`-bird-ready` and `-bird-live` from the readiness and liveness probes respectively

### Container image names

Instead of hardcoded image names and versions use placeholders to support configuration level settings. Following placeholders are used:

- `CalicoCNIImage` for calico/cni
- `CalicoNodeImage` for calico/node
- `CalicoKubeControllersImage` for calico/kube-controllers

Also, all containers in manifests were modified to have 'imagePullPolicy' field:

```yaml
imagePullPolicy: {{ .PullPolicy }}
```

Example:

```yaml
# calico-node.yaml
image: {{ .CalicoCNIImage }}
```