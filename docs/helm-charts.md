<!--
SPDX-FileCopyrightText: 2021 k0s authors
SPDX-License-Identifier: CC-BY-SA-4.0
-->

# Helm Charts

k0s provides built-in support for deploying applications using Helm charts. You can deploy charts using:

- **Chart custom resources** (recommended) - Self-contained Kubernetes resources that include repository configuration
- **k0s configuration file** - Centralized configuration for bootstrap and cluster-level management
- **Helm CLI** - Standard Helm commands work with k0s clusters

## Using Chart Custom Resources (Recommended)

The recommended way to deploy Helm charts in k0s is by creating `Chart` custom resources directly. This approach provides:

- **Self-contained configuration** - Each Chart includes its repository configuration
- **GitOps-friendly** - Manage charts as standard Kubernetes resources
- **Flexible credentials** - Different charts can use different repository credentials
- **Portable** - Charts can be moved between clusters easily
- **No k0s restart needed** - Apply changes instantly with `kubectl`

### Quick Start

Create a Chart resource to deploy Prometheus:

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
  values: |
    alertmanager:
      persistentVolume:
        enabled: false
  repository:
    url: https://prometheus-community.github.io/helm-charts
```

Apply it: `kubectl apply -f prometheus-chart.yaml`

### Chart Resource Configuration

#### Spec Fields

| Field          | Default value | Description                                                                    |
|----------------|---------------|--------------------------------------------------------------------------------|
| `chartName`    | _(required)_  | Chart reference: `repo/chart`, `oci://registry/chart`, or `/path/to/chart.tgz` |
| `version`      | _(required)_  | Chart version to install                                                       |
| `namespace`    | _(required)_  | Target namespace for the release                                               |
| `releaseName`  | -             | Helm release name (defaults to Chart resource name)                            |
| `values`       | -             | Custom chart values as YAML formatted string                                   |
| `timeout`      | `10m`         | Timeout to wait for release install                                            |
| `forceUpgrade` | `true`        | When set to `false`, disables the use of the `--force` flag when upgrading     |
| `repository`   | -             | Repository configuration (see below)                                           |

#### Repository Configuration

| Field      | Description                                                   |
|------------|---------------------------------------------------------------|
| `url`      | Repository URL (required for traditional repos, omit for OCI) |
| `username` | Username for Basic HTTP authentication                        |
| `password` | Password for Basic HTTP authentication                        |
| `caFile`   | CA bundle file path for HTTPS verification                    |
| `certFile` | TLS certificate file path for mTLS                            |
| `keyFile`  | TLS key file path for mTLS                                    |
| `insecure` | Skip TLS certificate checks (default: `false`)                |

**Note:** For traditional Helm repositories, only repositories providing a valid `index.yaml` are supported. See the [Helm repository documentation](https://helm.sh/docs/topics/chart_repository/) for details.

### Examples

#### Traditional Helm Repository

```yaml
apiVersion: helm.k0sproject.io/v1beta1
kind: Chart
metadata:
  name: nginx-ingress
  namespace: kube-system
spec:
  chartName: ingress-nginx/ingress-nginx
  version: "4.0.0"
  namespace: ingress-nginx
  repository:
    url: https://kubernetes.github.io/ingress-nginx
```

#### With Authentication

```yaml
apiVersion: helm.k0sproject.io/v1beta1
kind: Chart
metadata:
  name: private-chart
  namespace: kube-system
spec:
  chartName: myrepo/mychart
  version: "1.0.0"
  namespace: default
  repository:
    url: https://charts.example.com
    username: myuser
    password: mytoken
```

#### OCI Registry (Public)

```yaml
apiVersion: helm.k0sproject.io/v1beta1
kind: Chart
metadata:
  name: oci-chart
  namespace: kube-system
spec:
  chartName: oci://ghcr.io/org/chart
  version: "0.5.0"
  namespace: default
  # No repository field needed for public OCI registries
```

#### OCI Registry with Authentication

```yaml
apiVersion: helm.k0sproject.io/v1beta1
kind: Chart
metadata:
  name: private-oci-chart
  namespace: kube-system
spec:
  chartName: oci://registry.example.com:5000/charts/app
  version: "1.0.0"
  namespace: default
  repository:
    # URL not needed for OCI - it's in chartName
    username: robot-account
    password: secret-token
```

#### OCI Registry with Custom CA and mTLS

```yaml
apiVersion: helm.k0sproject.io/v1beta1
kind: Chart
metadata:
  name: registry-with-tls
  namespace: kube-system
spec:
  chartName: oci://registry.internal.com:8080/charts/app
  version: "1.0.0"
  namespace: default
  repository:
    caFile: /etc/k0s/pki/registry-ca.crt
    certFile: /etc/k0s/pki/client.crt
    keyFile: /etc/k0s/pki/client.key
```

#### Local Chart File

```yaml
apiVersion: helm.k0sproject.io/v1beta1
kind: Chart
metadata:
  name: local-chart
  namespace: kube-system
spec:
  chartName: /var/lib/k0s/charts/mychart-1.0.0.tgz
  version: "1.0.0"
  namespace: default
  # No repository field needed for local files
```

#### Secret-Based Authentication

For sensitive credentials, use Kubernetes Secrets instead of embedding them in Chart resources:

```bash
# Create a secret with repository credentials
kubectl create secret generic helm-registry-creds \
  --from-literal=username=myuser \
  --from-literal=password=mypassword \
  -n kube-system
```

```yaml
apiVersion: helm.k0sproject.io/v1beta1
kind: Chart
metadata:
  name: app-from-private-registry
  namespace: kube-system
spec:
  chartName: oci://registry.example.com/charts/app
  version: "1.0.0"
  namespace: default
  repository:
    configFrom:
      secretRef:
        name: helm-registry-creds
        # namespace defaults to Chart's namespace if omitted
```

**Secret Keys:**

- `url` - Repository URL
- `username` - HTTP Basic auth username  
- `password` - HTTP Basic auth password
- `ca.crt` - CA certificate bundle (PEM format)
- `tls.crt` - Client TLS certificate (PEM format)
- `tls.key` - Client TLS private key (PEM format)
- `insecure` - Set to "true" to skip TLS verification

**Precedence:** Secret values override inline repository fields when present. Inline fields provide defaults when corresponding secret keys are missing or empty.

**Example with defaults:**

```yaml
apiVersion: helm.k0sproject.io/v1beta1
kind: Chart
metadata:
  name: app-with-defaults
  namespace: kube-system
spec:
  chartName: oci://registry.example.com/charts/app
  version: "1.0.0"
  namespace: default
  repository:
    url: oci://registry-fallback.local/charts/app  # Used only if secret doesn't provide 'url'
    username: default-user                         # Used only if secret doesn't provide 'username'
    configFrom:
      secretRef:
        name: helm-registry-creds  # Secret values always override inline fields
```

### Chart Lifecycle

Charts are automatically managed by k0s:

- **Install**: When a Chart resource is created, k0s installs the Helm release
- **Upgrade**: When Chart spec changes (version, values), k0s upgrades the release
- **Uninstall**: When Chart resource is deleted, k0s uninstalls the release

Monitor chart status:

```bash
kubectl get charts -n kube-system
kubectl describe chart prometheus -n kube-system
```

**Note:** Chart resources must be created in the `kube-system` namespace for security reasons, but can deploy releases to any target namespace.

## Using k0s Configuration (Alternative Method)

For cluster-level Helm chart management during bootstrap or centralized configuration, you can define charts in the k0s configuration file. k0s automatically converts these into Chart custom resources.

This approach:

- Requires modifying `k0s.yaml` and restarting k0s controllers
- Useful for bootstrap and initial cluster setup
- Centralized configuration for all charts

### Configuration Structure

### Chart install and upgrade options

Charts are processed with the following Helm options by default:

- `--create-namespace`
- `--atomic`
- `--force` (only for the `upgrade` command)
- `--wait`
- `--wait-for-jobs`

See the configuration tables below for how to customize these options.

### Repository configuration

| Field      | Default value | Description                                                                                       |
|------------|---------------|---------------------------------------------------------------------------------------------------|
| `name`     | _(required)_  | The repository name                                                                               |
| `url`      | _(required)_  | The repository URL                                                                                |
| `insecure` | `true`        | Whether to skip TLS certificate checks when connecting to the repository                          |
| `caFile`   | -             | CA bundle file to use when verifying HTTPS-enabled servers                                        |
| `certFile` | -             | The TLS certificate file to use for HTTPS client authentication (mTLS)                            |
| `keyFile`  | -             | The TLS key file to use for HTTPS client authentication (mTLS)                                    |
| `username` | -             | Username for Basic HTTP authentication                                                            |
| `password` | -             | Password for Basic HTTP authentication                                                            |

**Note:** k0s supports only classic Helm chart repositories that provide a valid `index.yaml`. Direct links to chart folders or files (for example raw GitHub URLs) are not recognized as Helm repositories and will not work unless they follow the full Helm repository structure. For details on how a valid Helm chart repository must be structured, see: https://helm.sh/docs/topics/chart_repository/#create-a-chart-repository

### Chart configuration

| Field          | Default value | Description                                                                            |
|----------------|---------------|----------------------------------------------------------------------------------------|
| `name`         | -             | Release name                                                                           |
| `chartname`    | -             | Chart name in form `repository/chartname` or path to tgz file                          |
| `version`      | -             | Chart version to install                                                               |
| `timeout`      | -             | Timeout to wait for release install                                                    |
| `values`       | -             | Custom chart values as YAML formatted string                                           |
| `namespace`    | -             | Namespace to install the chart into                                                    |
| `forceUpgrade` | `true`        | When set to `false`, disables the use of the `--force` flag when upgrading the chart   |
| `order`        | `0`           | Order in which to apply the manifest. For equal values, alphanumeric ordering is used. |

### Example k0s.yaml Configuration

In this example, Prometheus is configured from the prometheus-community Helm chart repository:

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
        url: https://charts.example.com
        username: myuser
        password: mytoken
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
```

Add this to your `k0s.yaml` and restart k0s controllers. The charts will deploy automatically.

## Migrating from k0s Config to Chart Resources

To migrate from k0s.yaml configuration to Chart custom resources:

1. For each chart in your `spec.extensions.helm.charts` section:
   - Find the matching repository from `spec.extensions.helm.repositories`
   - Create a Chart resource with the repository configuration embedded
   - Apply the Chart resource: `kubectl apply -f chart.yaml`

2. Optionally remove charts from k0s.yaml (both methods can coexist)

**Example migration:**

**Before (k0s.yaml):**

```yaml
spec:
  extensions:
    helm:
      repositories:
      - name: prometheus-community
        url: https://prometheus-community.github.io/helm-charts
      charts:
      - name: prometheus
        chartname: prometheus-community/prometheus
        version: "14.6.1"
        namespace: default
```

**After (Chart resource):**

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
  repository:
    url: https://prometheus-community.github.io/helm-charts
```

## Popular Chart Examples

Example extensions that work well with k0s:

- Ingress controllers: [nginx ingress](https://github.com/helm/charts/tree/master/stable/nginx-ingress), [Traefik ingress](https://github.com/traefik/traefik-helm-chart) (refer to the k0s documentation for [Installing the Traefik Ingress Controller](examples/traefik-ingress.md))
- Volume storage providers: [OpenEBS](https://openebs.github.io/openebs/), [Rook](https://github.com/rook/rook/blob/master/Documentation/helm-operator.md), [Longhorn](https://longhorn.io/docs/0.8.1/deploy/install/install-with-helm/)
- Monitoring: [Prometheus](https://github.com/prometheus-community/helm-charts/), [Grafana](https://github.com/grafana/helm-charts)

## Helm debug logging

Running k0s controller with `--debug=true` enables helm debug logging.
