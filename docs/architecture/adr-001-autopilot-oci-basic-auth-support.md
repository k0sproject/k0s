# ADR 1: Add Support for OCI Registry and HTTP Authentication

## Context

Registries are increasingly being used as generic artifact stores, expanding beyond their traditional role of hosting container images. To align with this trend, it is beneficial for Autopilot to support pulling artifacts directly from registries. Currently, Autopilot's capabilities are limited to downloading artifacts via the HTTP[S] protocols.

Enhancing Autopilot to pull artifacts directly from registries will streamline workflows and improve efficiency by allowing integration and deployment of diverse artifacts without relying solely on HTTP[S] endpoints. This update will enable Autopilot to handle registry-specific protocols and authentication mechanisms, aligning it with modern deployment practices.

Currently, Autopilot does not support the retrieval of artifacts via the HTTP protocol when authentication is required. Implementing this feature to accommodate such authentication methods would be a valuable enhancement.

## Decision

Implement support in Autopilot for pulling artifacts, such as k0s binaries and image bundles, directly from a registry using the [ORAS](https://oras.land/docs/) client. Additionally, add support for HTTP authentication to ensure secure access to artifacts.

## Solution

Starting with the current `PlanResourceURL` struct:

```go
type PlanResourceURL struct {
        // URL is the URL of a downloadable resource.
        URL string `json:"url"`

        // Sha256 provides an optional SHA256 hash of the URL's content for verification.
        Sha256 string `json:"sha256,omitempty"`
}
```

We must specify to Autopilot where to access credentials for remote artifact pulls. This will be achieved by adjusting the struct as follows:

```go
type PlanResourceURL struct {
        // URL is the URL of a downloadable resource.
        URL string `json:"url"`

        // Sha256 provides an optional SHA256 hash of the URL's content for verification.
        Sha256 string `json:"sha256,omitempty"`

        // ArtifactPullSecrets holds a reference to a secret where the credentials are
        // stored. We use these credentials when pulling the artifacts from the provided
        // URL using any of the supported protocols (http, https, and oci).
        ArtifactPullSecret *ArtifactPullSecret `json:"artifactPullSecret,omitempty"`

        // InsecureSkipTLSVerify indicates whether certificates in the remote URL (if using
        // TLS) can be ignored.
        InsecureSkipTLSVerify bool  `json:"insecureSkipTLSVerify,omitempty"`
}
```

The `ArtifactPullSecret` property will be added and its struct will be defined as follow:

```go
type ArtifactPullSecret struct {
      // Namespace of the secret.
      Namespace string `json:"namespace"`

      // Name of the secret.
      Name string `json:"name"`
}
```

The secret pointed by the provided `ArtifactPullSecret` will be used for pulling artifacts using either HTTP[S] or OCI protocols and is expected to by of type `kubernetes.io/dockerconfigjson` if the protocol in use is `oci://` or of type `Opaque` if protocols `http://` or `https://` are used (see below for details on the Secret layout).

Example configuration for OCI:

```yaml
url: oci://my.registry/binaries/k0s:v1.30.1+k0s.0
sha256: e95603f167cce6e3cffef5594ef06785b3c1c00d3e27d8e4fc33824fe6c38a99
artifactPullSecret:
  namespace: kube-system
  name: artifacts-registry
```

Example configuration for HTTPS:

```yaml
url: https://my.file.server/binaries/k0s-v1.30.1+k0s.0
sha256: e95603f167cce6e3cffef5594ef06785b3c1c00d3e27d8e4fc33824fe6c38a99
artifactPullSecret:
  namespace: kube-system
  name: artifacts-basic-auth
```

Example configuration for HTTP:

```yaml
url: http://my.file.server/binaries/k0s-v1.30.1+k0s.0
sha256: e95603f167cce6e3cffef5594ef06785b3c1c00d3e27d8e4fc33824fe6c38a99
artifactPullSecret:
  namespace: kube-system
  name: artifacts-token-based-auth
```

### Secrets Layout

For secrets of type `kubernetes.io/dockerconfigjson` the format is the default for Docker authentications, equal to what is used in a Pod's pull secret. For further details you can refer to the [official documentation](https://kubernetes.io/docs/tasks/configure-pod-container/pull-image-private-registry/).

When it comes to the `Opaque` secret layout (used for HTTP requests) Autopilot will accept the following entries:

- `username` and `password`: if both are set then Autopilot will attempt to pull the artifacts using [Basic Authentication](https://www.ibm.com/docs/en/cics-ts/6.1?topic=concepts-http-basic-authentication).
- `authorization`: if this property is set then the `Authorization` [header](https://developer.mozilla.org/en-US/docs/Web/HTTP/Headers/Authorization) will be set to its value when pulling the artifacts.

No other property will be parsed and used. For sake of defining the expected behaviour in case of conflicting configurations:

> In the case where the three properties are set (`username`, `password`, and `authorization`) Autopilot will ignore `username` and `password`, i.e. `authorization`  takes precedence over username and password.

The `authorization` entry is used as is, its content is placed directly into the `Authorization` header. For example a secret like the following will make Autopilot to set the `Authorization` header to `Bearer abc123def456ghi789jkl0`:

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: creds
  namespace: kube-system
data:
  authorization: "Bearer abc123def456ghi789jkl0"
```

### Additional Details

- The `InsecureSkipTLSVerify` property is equivalent of defining `InsecureSkipTLSVerify` on a Go HTTP client.
- The `InsecureSkipTLSVerify` property will be valid for both `oci://` and `https://` protocols.
- If no protocol is defined, HTTP is used.
- If no `ArtifactPullSecret` is defined, access will be anonymous (no authentication).

## Status

Proposed

## Consequences

- Users will have an additional protocol to be aware of.
- Introduction of a new type (`ArtifactPullSecret`) when ideally existing types could be reused.
- If the Secret referenced by `ArtifactPullSecret` does not exist, the download will fail.
- Users need to be notified about different failure types (e.g., unreadable secret, invalid secret).
- Additional configuration is required to handle authentication, ensuring secure access to resources.
- We will allow downloads from remote places using self-signed certificates if requested to.
