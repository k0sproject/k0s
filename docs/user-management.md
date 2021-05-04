# User Management

### Adding a Cluster User

Run the [kubeconfig create](cli/k0s_kubeconfig_create.md) command on the controller to add a user to the cluster. The command outputs a kubeconfig for the user, to use for authentication.

```sh
$ k0s kubeconfig create [username]
```

### Enabling Access to Cluster Resources

Create the user with the `system:masters` group to grant the user access to the cluster:

```sh
$ k0s kubeconfig create --groups "system:masters" testUser > k0s.config
```

Create a `roleBinding` to grant the user access to the resources:

```sh
$ k0s kubectl create clusterrolebinding --kubeconfig k0s.config testUser-admin-binding --clusterrole=admin --user=testUser
```