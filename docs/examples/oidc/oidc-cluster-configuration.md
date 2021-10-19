# OpenID Connect integration

Developers use `kubectl` to access Kubernetes clusters. By default `kubectl` uses a certificate to authenticate to the Kubernetes API. This means that when multiple developers need to access a cluster, the certificate needs to be shared. Sharing the credentials to access a Kubernetes cluster presents a significant security problem. Compromise of the certificate is very easy and the consequences can be catastrophic.

In this tutorial, we walk through how to set up your Kubernetes cluster to add Single Sign-On support for kubectl using OpenID Connect (OIDC).

## OpenID Connect based authentication

OpenID Connect can be enabled by modifying k0s configuration (using extraArgs).

### Configuring k0s: overview

There are list of arguments for the kube-api that allows us to manage OIDC based authentication

| Parameter | Description | Example | Required |
| --------- | ----------- | ------- | ------- |
| `--oidc-issuer-url` | URL of the provider which allows the API server to discover public signing keys. Only URLs which use the `https://` scheme are accepted.  This is typically the provider's discovery URL without a path, for example "https://accounts.google.com" or "https://login.salesforce.com".  This URL should point to the level below .well-known/openid-configuration | If the discovery URL is `https://accounts.google.com/.well-known/openid-configuration`, the value should be `https://accounts.google.com` | Yes |
| `--oidc-client-id` |  A client id that all tokens must be issued for. | kubernetes | Yes |
| `--oidc-username-claim` | JWT claim to use as the user name. By default `sub`, which is expected to be a unique identifier of the end user. Admins can choose other claims, such as `email` or `name`, depending on their provider. However, claims other than `email` will be prefixed with the issuer URL to prevent naming clashes with other plugins. | sub | No |
| `--oidc-username-prefix` | Prefix prepended to username claims to prevent clashes with existing names (such as `system:` users). For example, the value `oidc:` will create usernames like `oidc:jane.doe`. If this flag isn't provided and `--oidc-username-claim` is a value other than `email` the prefix defaults to `( Issuer URL )#` where `( Issuer URL )` is the value of `--oidc-issuer-url`. The value `-` can be used to disable all prefixing. | `oidc:` | No |
| `--oidc-groups-claim` | JWT claim to use as the user's group. If the claim is present it must be an array of strings. | groups | No |
| `--oidc-groups-prefix` | Prefix prepended to group claims to prevent clashes with existing names (such as `system:` groups). For example, the value `oidc:` will create group names like `oidc:engineering` and `oidc:infra`. | `oidc:` | No |
| `--oidc-required-claim` | A key=value pair that describes a required claim in the ID Token. If set, the claim is verified to be present in the ID Token with a matching value. Repeat this flag to specify multiple claims. | `claim=value` | No |
| `--oidc-ca-file` | The path to the certificate for the CA that signed your identity provider's web certificate.  Defaults to the host's root CAs. | `/etc/kubernetes/ssl/kc-ca.pem` | No |

To set up bare minimum example we need to use:

- oidc-issuer-url
- oidc-client-id
- oidc-username-claim

### Configuring k0s: prerequisites

You will require:

- issuer-url
- client-id
- username-claim

Please, refer to [providers configuration guide](./oidc-provider-configuration.md) or your selected OIDC provider's own documentation (we don't cover all of them in k0s docs).

### Configuration example

```yaml
apiVersion: k0s.k0sproject.io/v1beta1
kind: ClusterConfig
spec:
  api:
    extraArgs:
      oidc-issuer-url: <issuer-url>
      oidc-client-id: <client-id>
      oidc-username-claim: email # we use email token claim field as a username
```

Use the configuration as a starting point. Continue with [configuration guide](../../configuration.md) for finishing k0s cluster installation.

## OpenID Connect based authorisation

There are two alternative options to implement authorization

### Provider based role mapping

Please refer to the [providers configuration guide](./oidc-provider-configuration.md). Generally speaking, using the `oidc-groups-claim` argument let's you specify which token claim is used a list of RBAC roles for a given user. You still need somehow sync up that data between your OIDC provider and kube-api RBAC system.

### Manual roles management

To use manual role management for each user you will need to create a role and role-binding for each new user within k0s cluster.
The role can be shared for all the users.
Role example:

```yaml
---
kind: Role
apiVersion: rbac.authorization.k8s.io/v1
metadata:
  namespace: default
  name: dev-role
rules:
- apiGroups: ["*"]
  resources: ["*"]
  verbs: ["*"]
```

RoleBinding example:

```yaml
kind: RoleBinding
apiVersion: rbac.authorization.k8s.io/v1
metadata:
  name: dev-role-binding
subjects:
  - kind: User
    name: <provider side user id>
roleRef:
  kind: Role
  name: dev-role
  apiGroup: rbac.authorization.k8s.io
```

The provided Role example is an all-inclusive and comprehensive example and should be tuned up to your actual requirements.

### kubeconfig management

NB: it's not safe to provide full content of the `/var/lib/k0s/pki/admin.conf` to the end-user. Instead, create a user specific kubeconfig with limited permissions.

The authorization side of the kubeconfig management is described in provider specific guides. Use `/var/lib/k0s/pki/admin.conf` as a template for cluster specific kubeconfig.

## References

[OAuth2 spec](https://oauth.net/2/)
[Kubernetes authorization system (RBAC)](https://kubernetes.io/docs/reference/access-authn-authz/rbac/)
[Kubernetes authenticating system](https://kubernetes.io/docs/reference/access-authn-authz/authentication/)
