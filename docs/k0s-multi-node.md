- [Creating a multi-node cluster](#creating-a-multi-node-cluster)
  - [Prerequisites](#prerequisites)
  - [Bootstrapping a controller node](#bootstrapping-a-controller-node)
  - [Create a join token](#create-a-join-token)
  - [Adding Workers to a Cluster](#adding-workers-to-a-cluster)
  - [Tokens](#tokens)
  - [Adding a Controller Node](#adding-a-controller-node)
  - [Adding a Cluster User](#adding-a-cluster-user)
    - [Enabling Access to Cluster Resources](#enabling-access-to-cluster-resources)
  - [Service and Log Setup](#service-and-log-setup)
  - [Configuring an HA Control Plane](#configuring-an-ha-control-plane)
    - [Requirements](#requirements)
        - [Load Balancer](#load-balancer)
        - [Cluster configuration](#cluster-configuration)
  - [Enabling Shell Completion](#enabling-shell-completion)
    - [Bash](#bash)
    - [Zsh](#zsh)
    - [Fish](#fish)


# Creating a multi-node cluster

As k0s binary has everything it needs packaged into a single binary, it makes it super easy to spin up Kubernetes clusters.

## Prerequisites

Install k0s as documented in the [installation instructions](install.md)


## Bootstrapping a controller node

Create a configuration file:

```sh
$ k0s default-config > k0s.yaml

```
If you wish to modify some of the settings, please check out the [configuration](configuration.md) documentation.

```sh
$ k0s install controller 
INFO[2021-02-25 15:34:59] Installing k0s service
$ systemctl start k0scontroller
```

k0s process will act as a "supervisor" for all of the control plane components. 
In a few seconds you'll have the control plane up-and-running.

## Create a join token

To be able to join workers into the cluster we need a token. The token embeds information with which we can enable mutual trust between the worker and controller(s) and allow the node to join the cluster as worker.

To get a token run the following on one of the existing controller nodes:
```sh
k0s token create --role=worker
```

This will output a long [token](#tokens) string, which we will then use to add a worker to the cluster. For enhanced security, we can also set an expiration time for the token by using:
```sh
$ k0s token create --role=worker --expiry=100h > token-file
```


## Adding Workers to a Cluster

To join the worker we need to run k0s in worker mode with the token from the previous step:
```sh
$ k0s install worker --token-file /path/to/token/file
```

That's it, really.

## Tokens

The tokens are actually base64 encoded [kubeconfigs](https://kubernetes.io/docs/tasks/access-application-cluster/configure-access-multiple-clusters/). 

Why:
- well defined structure
- can be used directly as bootstrap auth configs for kubelet
- embeds CA info for mutual trust

The actual bearer token embedded in the kubeconfig is a [bootstrap token](https://kubernetes.io/docs/reference/access-authn-authz/bootstrap-tokens/). For controller join token and for worker join token we use different usage attributes so we can make sure we can validate the token role on the controller side.


## Adding a Controller Node

To  add new controller nodes to the cluster, you must be using either etcd or an external data store (MySQL or Postgres) via kine. Please pay extra attention to the [HA Configuration](configuration.md#configuring-an-ha-control-plane) section in the configuration documentation, and make sure this configuration is identical for all controller nodes.

To create a join token for the new controller, run the following on an existing controller node:
```sh
$ k0s token create --role=controller --expiry=1h > token-file
```

On the new controller, run:
```sh
$ sudo k0s install controller --token-file /path/to/token/file
```

## Adding a Cluster User

To add a user to cluster, use the [kubeconfig create](cli/k0s_kubeconfig_create.md) command.
This will output a kubeconfig for the user, which can be used for authentication.

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

## Service and Log Setup
[k0s install](cli/k0s_install.md) sub-command was created as a helper command to allow users to easily install k0s as a service.
For more information, read [here](install.md).

## Configuring an HA Control Plane

The following pre-requisites are required in order to configure an HA control plane:
 
### Requirements
##### Load Balancer
A load balancer with a single external address should be configured as the IP gateway for the controllers.
The load balancer should allow traffic to each controller on the following ports:

- 6443
- 8132
- 8133
- 9443

##### Cluster configuration
On each controller node, a k0s.yaml configuration file should be configured.
The following options need to match on each node, otherwise the control plane components will end up in very unknown states:

- `network`
- `storage`: Needless to say, one cannot create a clustered controlplane with each node only storing data locally on SQLite.
- `externalAddress`

[Full configuration file refrence](configuration.md)
  

## Enabling Shell Completion
The k0s completion script for Bash, zsh, fish and powershell can be generated with the command `k0s completion < shell >`. Sourcing the completion script in your shell enables k0s autocompletion.
### Bash
```sh
echo 'source <(k0s completion bash)' >>~/.bashrc
```

```sh
# To load completions for each session, execute once:
$ k0s completion bash > /etc/bash_completion.d/k0s
```
### Zsh

If shell completion is not already enabled in your environment you will need
to enable it.  You can execute the following once:
```sh
$ echo "autoload -U compinit; compinit" >> ~/.zshrc
```
```sh
# To load completions for each session, execute once:
$ k0s completion zsh > "${fpath[1]}/_k0s"
```
You will need to start a new shell for this setup to take effect.

### Fish
```sh
$ k0s completion fish | source
```
```sh
# To load completions for each session, execute once:
$ k0s completion fish > ~/.config/fish/completions/k0s.fish
```
