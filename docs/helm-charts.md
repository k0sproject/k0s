<!--
SPDX-FileCopyrightText: 2021 k0s authors
SPDX-License-Identifier: CC-BY-SA-4.0
-->

# Helm Charts

Defining your extensions as Helm charts is one of two methods you can use to run k0s with your preferred extensions (the other being through the use of [Manifest Deployer](manifests.md)).

k0s supports two methods for deploying applications using Helm charts:

- Use Helm command in runtime to install applications. Refer to the [Helm Quickstart Guide](https://helm.sh/docs/intro/quickstart/) for more information.
- Insert Helm charts directly into the k0s configuration file, ``k0s.yaml``. This method does not require a separate install of `helm` tool and the charts automatically deploy at the k0s bootstrap phase.

## Helm charts in k0s configuration

Adding Helm charts into the k0s configuration file gives you a declarative way in which to configure the cluster. k0s controller manages the setup of Helm charts that are defined as extensions in the k0s configuration file.

### Chart install and upgrade options

Charts are processed the same way CLI tool does with following options by default:

- `--create-namespace`
- `--atomic`
- `--force` (only for the `upgrade` command)
- `--wait`
- `--wait-for-jobs`

See [Chart configuration](#chart-configuration) below for more details on how to configuring these options.

### Repository configuration

| Field      | Default value | Description                                                                                       |
|------------|---------------|---------------------------------------------------------------------------------------------------|
| `name`     | _(required)_  | The repository name                                                                               |
| `url`      | _(required)_  | The repository URL                                                                                |
| `insecure` | `true`        | Whether to skip TLS certificate checks when connecting to the repository                          |
| `caFile`   | -             | CA bundle file to use when verifying HTTPS-enabled servers                                        |
| `certFile` | -             | The TLS certificate file to use for HTTPS client authentication (not supported by OCI registries) |
| `keyfile`  | -             | The TLS key file to use for HTTPS client authentication (not supported by OCI registries)         |
| `username` | -             | Username for Basic HTTP authentication                                                            |
| `password` | -             | Password for Basic HTTP authentication                                                            |

### Chart configuration

| Field          | Default value | Description                                                                               |
|----------------|---------------|-------------------------------------------------------------------------------------------|
| `name`         | -             | Release name                                                                              |
| `chartname`    | -             | Chart name in form `repository/chartname` or path to tgz file                             |
| `version`      | -             | Chart version to install                                                                  |
| `timeout`      | -             | Timeout to wait for release install                                                       |
| `values`       | -             | Custom chart values as YAML formatted string                                              |
| `namespace`    | -             | Namespace to install the chart into                                                       |
| `forceUpgrade` | `true`        | When set to `false`, disables the use of the `--force` flag when upgrading the chart      |
| `order`        | `0`           | Order in which to to apply the manifest. For equal values, alphanumeric ordering is used. |

## Example

In the example, Prometheus is configured from "stable" Helms chart repository. Add the following to `k0s.yaml` and restart k0s, after which Prometheus should start automatically with k0s.

```yaml
spec:
  extensions:
    helm:
      concurrencyLevel: 5
      repositories:
      - name: stable
        url: https://charts.helm.sh/stable
      - name: prometheus-community
        url: https://prometheus-community.github.io/helm-charts
      - name: helm-repo-with-auth
        url: https://can-be-your-own-gitlab-ce-instance.org/api/v4/projects/PROJECTID/packages/helm/main
        username: access-token-name-as-username
        password: access-token-value-as-password
      - name: oci-registry-with-private-ca
        # OCI registry URL must not include any path elements
        url: oci://registry-with-private-ca.com:8080
        # Currently, only caFile is supported for TLS transport
        # Setting certFile or keyFile will result in an error
        caFile: /path/to/ca.crt
      charts:
      - name: prometheus-stack
        chartname: prometheus-community/prometheus
        version: "14.6.1"
        timeout: 20m
        order: 1
        values: |
          alertmanager:
            persistentVolume:
              enabled: false
          server:
            persistentVolume:
              enabled: false
        namespace: default
      # We don't need to specify the repo in the repositories section for OCI charts
      # unless a custom TLS transport is needed
      - name: oci-chart
        chartname: oci://registry:8080/chart
        version: "0.0.1"
        order: 2
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
      # the file must exist on all controller nodes
      - name: tgz-chart
        chartname: /tmp/chart.tgz
        version: "0.0.1"
        order: 2 
        values: ""
        namespace: default
```

Example extensions that you can use with Helm charts include:

- Ingress controllers: [nginx ingress](https://github.com/helm/charts/tree/master/stable/nginx-ingress), [Traefik ingress](https://github.com/traefik/traefik-helm-chart) (refer to the k0s documentation for [Installing the Traefik Ingress Controller](examples/traefik-ingress.md))
- Volume storage providers: [OpenEBS](https://openebs.github.io/charts/), [Rook](https://github.com/rook/rook/blob/master/Documentation/helm-operator.md), [Longhorn](https://longhorn.io/docs/0.8.1/deploy/install/install-with-helm/)
- Monitoring: [Prometheus](https://github.com/prometheus-community/helm-charts/), [Grafana](https://github.com/grafana/helm-charts)

## Helm debug logging

Running k0s controller with `--debug=true` enables helm debug logging.
