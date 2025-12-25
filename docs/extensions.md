<!--
SPDX-FileCopyrightText: 2020 k0s authors
SPDX-License-Identifier: CC-BY-SA-4.0
-->

# Cluster extensions

k0s allows users to use extensions to extend cluster functionality.

At the moment the only supported type of extensions is helm based charts.

The default configuration has no extensions.

## Helm based extensions

### Configuration example

```yaml
helm:
  repositories:
  - name: stable
    url: https://charts.helm.sh/stable
  - name: prometheus-community
    url: https://prometheus-community.github.io/helm-charts
  - name: oci-registry-with-private-ca
    # OCI registry URL must not include any path elements
    url: oci://registry-with-private-ca.com:8080
    # certFile and keyFile can be provided to enable mTLS
    caFile: /path/to/ca.crt
    certFile: /path/to/client.crt
    keyFile: /path/to/client.key
  charts:
  - name: prometheus-stack
    chartname: prometheus-community/prometheus
    version: "11.16.8"
    values: |
      storageSpec:
        emptyDir:
          medium: Memory
    namespace: default
  # We don't need to specify the repo in the repositories section for OCI charts
  # unless a custom TLS transport is needed
  - name: oci-chart
    chartname: oci://registry:8080/chart
    version: "0.0.1"
    values: ""
    namespace: default
  # OCI charts that require a custom TLS transport must add a repository entry 
  # pointing to TLS certificates on the controller node.
  # In this case, chartname of the chart must include the same registry URL 
  # previously defined in the repository URL.
  - name: oci-chart-with-tls
    chartname: oci://registry-with-private-ca.com:8080/chart
    version: "0.0.1"
    values: ""
    namespace: default
  # Other way is to use local tgz file with chart
  # the file must exist all controller nodes
  - name: tgz-chart
    chartname: /tmp/chart.tgz
    version: "0.0.1"
    values: ""
    namespace: default
```

By using the configuration above, the cluster would:

- add stable and prometheus-community chart repositories
- install the `prometheus-community/prometheus` chart of the specified version to the `default` namespace.

The chart installation is implemented by using CRD `helm.k0sproject.io/Chart`. For every given helm extension the cluster creates a Chart CRD instance. The cluster has a controller which monitors for the Chart CRDs, supporting the following operations:

- install
- upgrade
- delete

For security reasons, the cluster operates only on Chart CRDs instantiated in the `kube-system` namespace, however, the target namespace could be any.

#### CRD definition

```yaml
apiVersion: helm.k0sproject.io/v1beta1
kind: Chart
metadata:
  creationTimestamp: "2020-11-10T14:17:53Z"
  generation: 2
  labels:
    k0s.k0sproject.io/stack: helm
  name: k0s-addon-chart-test-addon
  namespace: kube-system
  resourceVersion: "627"
  selfLink: /apis/helm.k0sproject.io/v1beta1/namespaces/kube-system/charts/k0s-addon-chart-test-addon
  uid: ebe59ed4-1ff8-4d41-8e33-005b183651ed
spec:
  chartName: prometheus-community/prometheus
  namespace: default
  values: |
    storageSpec:
      emptyDir:
        medium: Memory
  version: 11.16.8
status:
  appVersion: 2.21.0
  namespace: default
  releaseName: prometheus-1605017878
  revision: 2
  updated: 2020-11-10 14:18:08.235656 +0000 UTC m=+41.871656901
  version: 11.16.8
```

The `Chart.spec` defines the chart information.

The `Chart.status` keeps the information about the last operation performed by the operator.
