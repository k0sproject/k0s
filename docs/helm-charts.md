# Helm Charts

Defining your extensions as Helm charts is one of two methods you can use to run k0s with your preferred extensions (the other being through the use of [Manifest Deployer](manifests.md)).

k0s supports two methods for deploying applications using Helm charts:

- Use Helm command in runtime to install applications. Refer to the [Helm Quickstart Guide](https://helm.sh/docs/intro/quickstart/) for more information.
- Insert Helm charts directly into the k0s configuration file, ``k0s.yaml``. This method does not require a separate install of `helm` tool and the charts automatically deploy at the k0s bootstrap phase.

## Helm charts in k0s configuration

Adding Helm charts into the k0s configuration file gives you a declarative way in which to configure the cluster. k0s controller manages the setup of Helm charts that are defined as extensions in the k0s configuration file.

### Wait for install

Each chart is proccesed the same way CLI tool does with following options:

- `--wait`
- `--wait-for-jobs`
- `--timeout 10m`

It is possible to customize timeout by using `.Timeout` field.

### Chart configuration

| Field     | Default value | Description                                                  |
|-----------|---------------|--------------------------------------------------------------|
| name      | -             | Release name                                                 |
| chartname | -             | chartname in form "repository/chartname" or path to tgz file |
| version   | -             | version to install                                           |
| timeout   | 10m           | timeout to wait for release install                          |
| values    | -             | yaml as a string, custom chart values                        |
| namespace | -             | namespace to install chart into                              |

## Example

In the example, Prometheus is configured from "stable" Helms chart repository. Add the following to `k0s.yaml` and restart k0s, after which Prometheus should start automatically with k0s.

```yaml
spec:
  extensions:
    helm:
      repositories:
      - name: stable
        url: https://charts.helm.sh/stable
      - name: prometheus-community
        url: https://prometheus-community.github.io/helm-charts
      charts:
      - name: prometheus-stack
        chartname: prometheus-community/prometheus
        version: "14.6.1"
        timeout: 20m
        values: |
          alertmanager:
            persistentVolume:
              enabled: false
          server:
            persistentVolume:
              enabled: false
        namespace: default
      # We don't need to specify the repo in the repositories section for OCI charts
      - name: oci-chart
        chartname: oci://registry:8080/chart
        version: "0.0.1"
        values: ""
        namespace: default
      # Other way is to use local tgz file with chart
      # the file must exist on all controller nodes
      - name: tgz-chart
        chartname: /tmp/chart.tgz
        version: "0.0.1"
        values: ""
        namespace: default
```

Example extensions that you can use with Helm charts include:

- Ingress controllers: [Nginx ingress](https://github.com/helm/charts/tree/master/stable/nginx-ingress), [Traefix ingress](https://github.com/traefik/traefik-helm-chart) (refer to the k0s documentation for [Installing the Traefik Ingress Controller](examples/traefik-ingress.md))
- Volume storage providers: [OpenEBS](https://openebs.github.io/charts/), [Rook](https://github.com/rook/rook/blob/master/Documentation/helm-operator.md), [Longhorn](https://longhorn.io/docs/0.8.1/deploy/install/install-with-helm/)
- Monitoring: [Prometheus](https://github.com/prometheus-community/helm-charts/), [Grafana](https://github.com/grafana/helm-charts)

## Helm debug logging

Running k0s controller with `--debug=true` enables helm debug logging.
