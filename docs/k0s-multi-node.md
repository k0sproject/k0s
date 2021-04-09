# Manual Install (for advanced users)

In this tutorial you'll create a multi-node cluster, which is locally managed in each node. It requires several steps to install each node separately and connect the nodes together with the access tokens. This tutorial is targeted for advanced users who want to setup their k0s nodes manually.

### Prerequisites

This tutorial has been written for Debian/Ubuntu, but it can be used for any Linux running one of the supported init systems: Systemd or OpenRC.

Before proceeding, make sure to review the [System Requirements](system-requirements.md).

To speed-up the usage of `k0s` command, you may want to enable [shell completion](shell-completion.md).

### Installation steps

#### 1. Download k0s

The k0s download script downloads the latest stable k0s and makes it executable from /usr/bin/k0s.
```
$ curl -sSLf https://get.k0s.sh | sudo sh
```
The download script accepts the following environment variables:

1. `K0S_VERSION=v0.11.0` - select the version of k0s to be installed
2. `DEBUG=true` - outputs commands and their arguments as they are executed.

If you need to use environment variables and you use sudo, you may need `--preserve-env` like
```sh
curl -sSLf https://get.k0s.sh | sudo --preserve-env=K0S_VERSION sh
```

#### 2. Bootstrap a controller node

Create a configuration file:

```sh
$ k0s default-config > k0s.yaml

```
If you wish to modify some of the settings, please check out the [configuration](configuration.md) documentation.

```sh
$ k0s install controller -c k0s.yaml
```
```sh
$ systemctl start k0scontroller
```

k0s process will act as a "supervisor" for all of the control plane components. In a few seconds you'll have the control plane up-and-running.

#### 3. Create a join token

To be able to join workers into the cluster a token is needed. The token embeds information, which enables mutual trust between the worker and controller(s) and allows the node to join the cluster as worker.

To get a token run the following command on one of the existing controller nodes:
```sh
$ k0s token create --role=worker
```

This will output a long [token](#tokens) string, which you will use to add a worker to the cluster. For enhanced security, it's possible to set an expiration time for the token by using:
```sh
$ k0s token create --role=worker --expiry=100h > token-file
```

#### 4. Add workers to the cluster

To join the worker we need to run k0s in the worker mode with the token from the previous step:
```sh
$ k0s install worker --token-file /path/to/token/file
```
```sh
$ systemctl start k0sworker
```

##### About tokens

The tokens are actually base64 encoded [kubeconfigs](https://kubernetes.io/docs/tasks/access-application-cluster/configure-access-multiple-clusters/). 

Why:

- Well defined structure
- Can be used directly as bootstrap auth configs for kubelet
- Embeds CA info for mutual trust

The actual bearer token embedded in the kubeconfig is a [bootstrap token](https://kubernetes.io/docs/reference/access-authn-authz/bootstrap-tokens/). For controller join token and for worker join token we use different usage attributes so we can make sure we can validate the token role on the controller side.

#### 5. Add controllers to the cluster

To add new controller nodes to the cluster, you must be using either etcd or an external data store (MySQL or Postgres) via kine. Please pay an extra attention to the [high availability configuration](high-availability.md), and make sure this configuration is identical for all controller nodes.

To create a join token for the new controller, run the following on an existing controller:
```sh
$ k0s token create --role=controller --expiry=1h > token-file
```

On the new controller, run:
```sh
$ sudo k0s install controller --token-file /path/to/token/file
```
```sh
$ systemctl start k0scontroller
```

#### 6. Check service and k0s status

You can check the service status and logs like this:
```
$ sudo systemctl status k0scontroller
     Loaded: loaded (/etc/systemd/system/k0scontroller.service; enabled; vendor preset: enabled)
     Active: active (running) since Fri 2021-02-26 08:37:23 UTC; 1min 25s ago
       Docs: https://docs.k0sproject.io
   Main PID: 1408647 (k0s)
      Tasks: 96
     Memory: 1.2G
     CGroup: /system.slice/k0scontroller.service
     ....
```

To get general information about your k0s instance:
```
$ sudo k0s status
Version: v0.11.0
Process ID: 436
Parent Process ID: 1
Role: controller
Init System: linux-systemd
```

#### 7. Access your cluster

The Kubernetes command-line tool 'kubectl' is included into k0s binary. You can use it for example to deploy your application or check your node status like this:
```
$ sudo k0s kubectl get nodes
NAME   STATUS   ROLES    AGE    VERSION
k0s    Ready    <none>   4m6s   v1.21.0-k0s1
```

You can also access your cluster easily with [LENS](https://k8slens.dev/). Just copy the kubeconfig 
```sh
sudo cat /var/lib/k0s/pki/admin.conf
```
and paste it to LENS. Note that in the kubeconfig you need add your controller's host ip address to the server field (replacing localhost) in order to access the cluster from an external network.

### Next Steps

- [Installing with k0sctl](k0sctl-install.md) for deploying and upgrading multi-node clusters with one command
- [Control plane configuration options](configuration.md) for example for networking and datastore configuration
- [Worker node configuration options](worker-node-config.md) for example for node labels and kubelet arguments
- [Support for cloud providers](cloud-providers.md) for example for load balancer or storage configuration
- [Installing the Traefik Ingress Controller](examples/traefik-ingress.md), a tutorial for ingress deployment
