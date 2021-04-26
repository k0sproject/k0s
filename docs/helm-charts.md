# Helm Charts

Helm charts is one of two methods you can use to run k0s with your preferred extentions (the other being [Manifest Deployer](manifests.md), which is included with k0s).

k0s supports two methods for deploying applications using Helm charts:

- Use Helm command in runtime to install applications. Refer to the [Helm Quickstart Guide](https://helm.sh/docs/intro/quickstart/) for more information.
- Insert Helm charts directly into the k0s configuration file, ``k0s.yaml``. This method does not require a separate install of `helm` tool and the charts automatically deploy at the k0s bootstrap phase.

### Helm charts in k0s configuration

Adding Helm charts into the k0s configuration file gives you a declarative way in which to configure the cluster. k0s controller manages the setup of the defined extension Helm charts as part of the cluster bootstrap process.

### Example

In the example, Prometheus is configured from "stable" Helms chart repository.
Add the following to ``k0s.yaml`` and restart k0s, after which Prometheus
should start automatically with k0s.

```sh
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
          version: "11.16.8"
          values: |
            storageSpec:
              emptyDir:
                medium: Memory
          namespace: default
```

Example extensions that you can use with Helm charts include:

- Ingress controllers: [Nginx
  ingress](https://github.com/helm/charts/tree/master/stable/nginx-ingress),
  [Traefix ingress](https://github.com/traefik/traefik-helm-chart) (refer
  to the k0s documentation for [Installing the Traefik Ingress Controller](examples/traefik-ingress.md))
- Volume storage providers: [OpenEBS](https://openebs.github.io/charts/), [Rook](https://github.com/rook/rook/blob/master/Documentation/helm-operator.md), [Longhorn](https://longhorn.io/docs/0.8.1/deploy/install/install-with-helm/)
- Monitoring: [Prometheus](https://github.com/prometheus-community/helm-charts/), [Grafana](https://github.com/grafana/helm-charts)
