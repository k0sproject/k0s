# Upgrading Calico

MKE bundles Kubernetes manifests for Calico. The manifests are retrieved
from the [official Calico docs](https://docs.projectcalico.org/manifests/calico.yaml).

As fetching and modifying the entire multi-thousand line file is error-prone,
you may follow these steps to upgrade Calico to the latest version:

1. run `./get-calico.sh`
2. check the git diff to see if it looks sensible
3. re-apply our manual adjustments (documented below)
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
- variable-based cluster CIDR:  
```helmyaml
- name: CALICO_IPV4POOL_CIDR
  value: "{{ .ClusterCIDR }}"
```

## Extending to bundle more manifests

If we have a future need to bundle additional manifests, we need to make
sure to move the `bindata` generation out of `get-calico` into a separate
step that can bundle all manifests into a single bindata file within the
`static` package.
