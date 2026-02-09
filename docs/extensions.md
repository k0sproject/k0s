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

The chart installation is implemented by using the `helm.k0sproject.io/Chart` custom resource. k0s automatically converts charts defined in the k0s configuration into Chart resources. You can also create Chart resources directly for more flexible management.

The cluster has a controller which monitors Chart resources and supports the following operations:

- install
- upgrade
- delete

For security reasons, the cluster operates only on Chart resources in the `kube-system` namespace, however, the target namespace for releases can be any namespace.

#### Chart Resource Specification

```yaml
apiVersion: helm.k0sproject.io/v1beta1
kind: Chart
metadata:
  name: prometheus
  namespace: kube-system
spec:
  chartName: prometheus-community/prometheus
  version: "14.6.1"
  namespace: default
  timeout: 20m
  values: |
    alertmanager:
      persistentVolume:
        enabled: false
  repository:
    url: https://prometheus-community.github.io/helm-charts
status:
  releaseName: prometheus
  version: "14.6.1"
  appVersion: "2.31.1"
  revision: 1
  namespace: default
  updated: "2024-01-15T10:30:45Z"
```

The `spec.repository` field allows each Chart to include its own repository configuration, making Charts self-contained and portable. When using k0s configuration file, k0s automatically embeds the repository configuration into the generated Chart resources.

For more details on using Chart resources directly, see the [Helm Charts](helm-charts.md) documentation.
