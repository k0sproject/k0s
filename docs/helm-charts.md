# Helm Charts

k0s supports deploying applications with Helm charts in two ways:

- Helm command can be used in runtime to install applications like described in the [Helm Quickstart Guide](https://helm.sh/docs/intro/quickstart/).
- Helm charts can be inserted directly into the k0s configuration file (k0s.yaml). This way a separate install of `helm` tool is not needed and the charts are automatically deployed at the k0s bootstrap phase.

### Helm charts in k0s configuration

When Helm charts are added into k0s configuration file, you'll get a declarative way to configure the cluster. k0s controller manages the setup of the defined extension Helm charts as part of the cluster bootstrap process.

### Example

This example configures Prometheus from "stable" Helms chart repository. Just add the following to your k0s configuration file (k0s.yaml) and restart k0s. Prometheus should now start automatically with k0s.

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

Some example extensions that you could use with Helm charts:

- Ingress controllers: [Nginx ingress](https://github.com/helm/charts/tree/master/stable/nginx-ingress), [Traefix ingress](https://github.com/traefik/traefik-helm-chart) ([tutorial](examples/traefik-ingress.md))
- Volume storage providers: [OpenEBS](https://openebs.github.io/charts/), [Rook](https://github.com/rook/rook/blob/master/Documentation/helm-operator.md), [Longhorn](https://longhorn.io/docs/0.8.1/deploy/install/install-with-helm/)
- Monitoring: [Prometheus](https://github.com/prometheus-community/helm-charts/), [Grafana](https://github.com/grafana/helm-charts)
