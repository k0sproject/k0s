# Creating A cluster with k0s

As k0s binary has everything it needs packaged into a single binary, it makes it super easy to spin up Kubernetes clusters.

## Pre-requisites

Download k0s binary from [releases](https://github.com/k0sproject/k0s/releases/latest) and push it to all the nodes you wish to connect to the cluster.

That's it, really.

## Bootstrapping controller node

Create a [configuration](configuration.md) file if you wish to tune some of the settings.

```
$ k0s server -c k0s.yaml
```

That's it, really. k0s process will act as a "supervisor" for all the control plane components. In few seconds you'll have the control plane up-and-running.

Naturally, to make k0s boot up the control plane when the node itself reboots you should really make the k0s process to be supervised by systemd or some other init system.

## Create join token

To be able to join workers into the cluster we need a token. The token embeds information with which we can enable mutual trust between the worker and controller(s) and allow the node to join the cluster as worker.

To get a token run the following on one of the existing controller nodes:
```sh
k0s token create --role=worker
```

This will output a long [token](#tokens) which we will use to join the worker. To enhance security, we can also set an expiration time on the tokens by using:
```sh
k0s token create --role=worker --expiry="100h"
```


## Joining worker(s) to cluster

To join the worker we need to run k0s in worker mode with the token from previous step:
```sh
$ k0s worker "long-join-token"
```

That's it, really.

Naturally, to make k0s boot up the worker components when the node itself reboots you should really make the k0s process to be supervised by systemd or some other init system.

## Tokens

The tokens are actually base64 encoded [kubeconfigs](https://kubernetes.io/docs/tasks/access-application-cluster/configure-access-multiple-clusters/). 

Why:
- well defined structure
- can be used directly as bootstrap auth configs for kubelet
- embeds CA info for mutual trust

The actual bearer token embedded in the kubeconfig is a [bootstrap token](https://kubernetes.io/docs/reference/access-authn-authz/bootstrap-tokens/). For controller join token and for worker join token we use different usage attributes so we can make sure we can validate the token role on the controller side.


## Join controller node

To be able to join a new controller node into the cluster you must be using either etcd or some externalized data store (MySQL or Postgres) via kine. Also make sure the [configurations](configuration.md) match for the data storage on all controller nodes.

To create a join token for the new controller, run the following on existing controller node:
```sh
k0s token create --role=controller --expiry=1h
```

On the new controller, run:
```sh
k0s server "long-join-token"
```

## Adding a Cluster User

To add a user to cluster, use the [kubeconfig create](cli/k0s_kubeconfig_create.md) command.
This will output a kubeconfig for the user, which can be used for authentication.

On the controller, run the following to generate a kubeconfig for a user :
```shell script
k0s kubeconfig create [username]
```
### Enabling Access to Cluster Resources
To allow the user access to the cluster, the user needs to be created with the `system:masters` group:

```shell script
clusterUser="testUser"
k0s kubeconfig create --groups "system:masters" $clusterUser > ~/.kube/config
```
Create the proper roleBinding, to allow the user access to the resources:
```shell script
kubectl create clusterrolebinding $clusterUser-admin-binding --clusterrole=admin --user=$clusterUser
```

## Service and Log Setup
[k0s install](cli/k0s_install.md) sub-command was created as a helper command to allow users to easily install k0s as a service.
For more information, read [here](install.md).