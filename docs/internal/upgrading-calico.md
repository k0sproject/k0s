# Upgrading Calico

MKE bundles Kubernetes manifests for Calico. The manifests are retrieved
from the [official Calico docs](https://docs.projectcalico.org/manifests/calico.yaml).

As fetching and modifying the entire multi-thousand line file is error-prone,
you may follow these steps to upgrade Calico to the latest version:

1. run `./get-calico.sh`
1. check the git diff to see if it looks sensible
1. re-apply our manual adjustments (documented below)
1. run `make bindata-manifests`
4. compile, pray, and test
5. commit and create a PR

## Manual Adjustments

**Note:** All manual adjustments should be fairly obvious from the git diff.
This section attempts to provide a sanity checklist to go through and make sure
we still have those changes applied. The code blocks in this section are **our modifications**,
not the calico originals.

`static/manifests/calico/DaemonSet/calico-node.yaml`:

- variable-based support for both vxlan and ipip (search for `ipip` to find):  
```helmyaml
{{- if eq .Mode "ipip" }}
# Enable IPIP
- name: CALICO_IPV4POOL_IPIP
  value: "Always"
# Enable or Disable VXLAN on the default IP pool.
- name: CALICO_IPV4POOL_VXLAN
  value: "Never"
{{- else if eq .Mode "vxlan" }}
# Disable IPIP
- name: CALICO_IPV4POOL_IPIP
  value: "Never"
# Enable VXLAN on the default IP pool.
- name: CALICO_IPV4POOL_VXLAN
  value: "Always"
- name: FELIX_VXLANPORT
  value: "{{ .VxlanPort }}"
- name: FELIX_VXLANVNI
  value: "{{ .VxlanVNI }}"
{{- end }}
```

- variable-based WireGuard support:
```helmyaml
{{- if .EnableWireguard }}
- name: FELIX_WIREGUARDENABLED
  value: "true"
{{- end }}
```
- variable-based cluster CIDR:  
```helmyaml
- name: CALICO_IPV4POOL_CIDR
  value: "{{ .ClusterCIDR }}"
```
- custom backend and MTU
```helmyaml
# calico-config.yaml
calico_backend: "{{ .Mode }}"
veth_mtu: "{{ .MTU }}"
```
- remove bgp from `CLUSTER_TYPE`
```helmyaml
- name: CLUSTER_TYPE
  value: "k8s"
```
- disable BIRD checks on liveness and readiness as we don't support BGP by removing
`-bird-ready` and `-bird-live` from the readiness and liveness probes respectively

### Container image names 

Instead of hardcoded image names and versions use placeholders to support configuration level settings. Following placeholders are used:

- `CalicoCNIImage` for calico/cni
- `CalicoFlexVolumeImage` for calico/pod2daemon-flexvol
- `CalicoNodeImage` for calico/node
- `CalicoKubeControllersImage` for calico/kube-controllers


Example: 
```
# calico-node.yaml
image: {{ .CalicoCNIImage }}
```