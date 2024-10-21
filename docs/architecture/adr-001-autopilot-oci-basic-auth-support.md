# ADR 1: Add Support for OCI Registry and HTTP Authentication

## Context

Registries are increasingly being used as generic artifact stores, expanding
beyond their traditional role of hosting container images. To align with this
trend, it is beneficial for Autopilot to support pulling artifacts directly
from registries. Currently, Autopilot's capabilities are limited to downloading
artifacts via the HTTP\[S\] protocols.

Enhancing Autopilot to pull artifacts directly from registries will streamline
workflows and improve efficiency by allowing integration and deployment of
diverse artifacts without relying solely on HTTP\[S\] endpoints. This update
will enable Autopilot to handle registry-specific protocols and authentication
mechanisms, aligning it with modern deployment practices.

Currently, Autopilot does not support the retrieval of artifacts via the HTTP
protocol when authentication is required. Implementing this feature to
accommodate such authentication methods would be a valuable enhancement.

## Decision

Implement support in Autopilot for pulling artifacts, such as k0s binaries and
image bundles, directly from a registry using the
[ORAS](https://oras.land/docs/) client. Additionally, add support for HTTP
authentication to ensure secure access to artifacts.

## Solution

Starting with the current `PlanResourceURL` struct:

```go
type PlanResourceURL struct {
	// URL is the URL of a downloadable resource.
	URL string `json:"url"`

	// Sha256 provides an optional SHA256 hash of the URL's content for
	// verification.
	Sha256 string `json:"sha256,omitempty"`
}
```

We must specify to Autopilot where to access credentials for remote artifact
pulls. This will be achieved by adjusting the struct as follows:

```go
type PlanResourceURL struct {
	// URL is the URL of a downloadable resource.
	URL string `json:"url"`

	// Sha256 provides an optional SHA256 hash of the URL's content for
	// verification.
	Sha256 string `json:"sha256,omitempty"`

	// SecretRef holds a reference to a secret where the credentials are
	// stored. We use these credentials when pulling the artifacts from the
	// provided URL using
	// any of the supported protocols (http, https, and oci).
	SecretRef *corev1.SecretReference `json:"secretRef,omitempty"`

	// InsecureSkipTLSVerify indicates whether certificates in the remote
	// URL (if using TLS) can be ignored.
	InsecureSkipTLSVerify bool `json:"insecureSkipTLSVerify,omitempty"`
}
```

`SecretRef` property is of type `SecretReference` as defined by
`k8s.io/api/core/v1` package. The secret pointed by the provided `SecretRef`
will be used for pulling artifacts using either HTTP\[S\] or OCI protocols.

### Example Configurations

#### Configuration for OCI

```yaml
url: oci://my.registry/binaries/k0s:v1.30.1+k0s.0
sha256: e95603f167cce6e3cffef5594ef06785b3c1c00d3e27d8e4fc33824fe6c38a99
secretRef:
  namespace: kube-system
  name: artifacts-registry
```

#### Configuration for OCI using plain HTTP transport

```yaml
url: oci+http://my.registry/binaries/k0s:v1.30.1+k0s.0
sha256: e95603f167cce6e3cffef5594ef06785b3c1c00d3e27d8e4fc33824fe6c38a99
secretRef:
  namespace: kube-system
  name: artifacts-registry
```

#### Configuration for HTTPS

```yaml
url: https://my.file.server/binaries/k0s-v1.30.1+k0s.0
sha256: e95603f167cce6e3cffef5594ef06785b3c1c00d3e27d8e4fc33824fe6c38a99
secretRef:
  namespace: kube-system
  name: artifacts-basic-auth
```

#### Configuration for HTTP

```yaml
url: http://my.file.server/binaries/k0s-v1.30.1+k0s.0
sha256: e95603f167cce6e3cffef5594ef06785b3c1c00d3e27d8e4fc33824fe6c38a99
secretRef:
  namespace: kube-system
  name: artifacts-token-based-auth
```

### Secrets Layout

The following standard Kubernetes secret types are supported:

- [`kubernetes.io/basic-auth`](https://kubernetes.io/docs/concepts/configuration/secret/#basic-authentication-secret)<br>
  The username and password are used according to the protocol's standard
  procedure for password-based authentication.

- [`kubernetes.io/dockerconfigjson`](https://kubernetes.io/docs/concepts/configuration/secret/#docker-config-secrets)<br>
   It works in the same way as a Pod's [image pull secret]. Only supported for
  the `oci://` protocol. (Might be supported for other protocols in the future,
  as well).

[image pull secret]: https://kubernetes.io/docs/tasks/configure-pod-container/pull-image-private-registry/

Potentially supported in the future:

- [`kubernetes.io/tls`](https://kubernetes.io/docs/concepts/configuration/secret/#tls-secrets)<br>
  For TLS client authentication.

Moreover, k0s supports the following custom secret type:

- `k0sproject.io/http-authorization-header`<br>
  Sets a custom value for the HTTP Authorization header:

  ```yaml
  apiVersion: v1
  kind: Secret
  metadata:
    name: creds
    namespace: kube-system
  data:
    authorization: "Bearer abc123def456ghi789jkl0"
  ```

  The `authorization` entry is used as is, with its content placed directly into
  the `Authorization` header. A secret like the above will make Autopilot set
  the `Authorization` header to `Bearer abc123def456ghi789jkl0`.

### Additional Details

- The `InsecureSkipTLSVerify` property is equivalent to defining
  `InsecureSkipTLSVerify` on a Go HTTP client.
- The `InsecureSkipTLSVerify` property will be valid for both `oci://` and
  `https://` protocols. It has no effect for the `oci+http://` and `http://`
  protocols.
- If a protocol is not specified or an incorrect one is provided, an error
  state should be activated.
- If no `SecretRef` is defined, access will be anonymous (no authentication).

## Status

Proposed

## Consequences

- Users will have an additional protocol to be aware of.
- If the Secret referenced by `SecretRef` does not exist, the download will
  fail.
- Users need to be notified about different failure types (e.g., unreadable
  secret, invalid secret).
- Additional configuration is required to handle authentication, ensuring
  secure access to resources.
- We will allow downloads from remote places using self-signed certificates if
  requested to.
