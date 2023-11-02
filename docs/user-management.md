# User Management

Kubernetes, and thus k0s, does not have any built-in functionality to manage users. Kubernetes relies solely on external sources for user identification and authentication. A client certificate is considered an external source in this case as Kubernetes api-server "just" validates that the certificate is signed by a trusted CA. This means that it is recommended to use e.g. [OpenID Connect](./examples/oidc/oidc-cluster-configuration.md) to configure the API server to trust tokens issued by an external Identity Provider.

k0s comes with some helper commands to create kubeconfig with client certificates for users. There are few caveats that one needs to take into consideration when using client certificates:

* Client certificates have long expiration time, they're valid for one year
* Client certificates cannot be revoked (general Kubernetes challenge)

## Adding a Cluster User

Run the [kubeconfig create](cli/k0s_kubeconfig_create.md) command on the controller to add a user to the cluster. The command outputs a kubeconfig for the user, to use for authentication.

```shell
k0s kubeconfig create [username]
```

## Enabling Access to Cluster Resources

Create the user with the `system:masters` group to grant the user access to the cluster:

```shell
k0s kubeconfig create --groups "system:masters" testUser > k0s.config
```

Create a `roleBinding` to grant the user access to the resources:

```shell
k0s kubectl create clusterrolebinding --kubeconfig k0s.config testUser-admin-binding --clusterrole=admin --user=testUser
```
