# User Management

### Adding a Cluster User

To add a user to the cluster, use the [kubeconfig create](cli/k0s_kubeconfig_create.md) command. This will output a kubeconfig for the user, which can be used for authentication.

On the controller, run the following to generate a kubeconfig for a user:

```sh
$ k0s kubeconfig create [username]
```

### Enabling Access to Cluster Resources
To allow the user access to the cluster, the user needs to be created with the `system:masters` group:
```sh
$ k0s kubeconfig create --groups "system:masters" testUser > k0s.config
```

Create a `roleBinding` to grant the user access to the resources:
```sh
$ k0s kubectl create clusterrolebinding --kubeconfig k0s.config testUser-admin-binding --clusterrole=admin --user=testUser
```