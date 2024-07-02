# Manual Install (Advanced)

You can manually set up k0s nodes by creating a multi-node cluster that is locally managed on each node. This involves several steps, to first install each node separately, and to then connect the node together using access tokens.

## Prerequisites

**Note**: Before proceeding, make sure to review the [System Requirements](system-requirements.md).

Though the Manual Install material is written for Debian/Ubuntu, you can use it for any Linux distro that is running either a Systemd or OpenRC init system.

You can speed up the use of the `k0s` command by enabling [shell completion](shell-completion.md).

## Install k0s

### 1. Download k0s

Run the k0s download script to download the latest stable version of k0s and make it executable from /usr/bin/k0s.

```shell
curl --proto '=https' --tlsv1.2 -sSf https://get.k0s.sh | sudo sh
```

The download script accepts the following environment variables:

| Variable                    | Purpose                                                              |
|:----------------------------|:---------------------------------------------------------------------|
| `K0S_VERSION=v{{{ extra.k8s_version }}}+k0s.0` | Select the version of k0s to be installed         |
| `DEBUG=true`                                   | Output commands and their arguments at execution. |

**Note**: If you require environment variables and use sudo, you can do:

```shell
curl --proto '=https' --tlsv1.2 -sSf https://get.k0s.sh | sudo K0S_VERSION=v{{{ extra.k8s_version }}}+k0s.0 sh
```

### 2. Bootstrap a controller node

Create a configuration file:

```shell
mkdir -p /etc/k0s
k0s config create > /etc/k0s/k0s.yaml
```

**Note**: For information on settings modification, refer to the [configuration](configuration.md) documentation.

```shell
sudo k0s install controller -c /etc/k0s/k0s.yaml
```

```shell
sudo k0s start
```

k0s process acts as a "supervisor" for all of the control plane components. In moments the control plane will be up and running.

### 3. Create a join token

You need a token to join workers to the cluster. The token embeds information that enables mutual trust between the worker and controller(s) and which allows the node to join the cluster as worker.

To get a token, run the following command on one of the existing controller nodes:

```shell
sudo k0s token create --role=worker
```

The resulting output is a long [token](#about-tokens) string, which you can use to add a worker to the cluster.

For enhanced security, run the following command to set an expiration time for the token:

```shell
sudo k0s token create --role=worker --expiry=100h > token-file
```

### 4. Add workers to the cluster

To join the worker, run k0s in the worker mode with the join token you created:

```shell
sudo k0s install worker --token-file /path/to/token/file
```

```shell
sudo k0s start
```

#### About tokens

The join tokens are base64-encoded [kubeconfigs](https://kubernetes.io/docs/tasks/access-application-cluster/configure-access-multiple-clusters/) for several reasons:

- Well-defined structure
- Capable of direct use as bootstrap auth configs for kubelet
- Embedding of CA info for mutual trust

The bearer token embedded in the kubeconfig is a [bootstrap token](https://kubernetes.io/docs/reference/access-authn-authz/bootstrap-tokens/). For controller join tokens and worker join tokens k0s uses different usage attributes to ensure that k0s can validate the token role on the controller side.

### 5. Add controllers to the cluster

**Note**: Either etcd or an external data store (MySQL or Postgres) via kine must be in use to add new controller nodes to the cluster. Pay strict attention to the [high availability configuration](high-availability.md) and make sure the configuration is identical for all controller nodes.

To create a join token for the new controller, run the following command on an existing controller:

```shell
sudo k0s token create --role=controller --expiry=1h > token-file
```

On the new controller, run:

```shell
sudo k0s install controller --token-file /path/to/token/file -c /etc/k0s/k0s.yaml
```

Important notice here is that each controller in the cluster must have k0s.yaml otherwise some cluster nodes will use default config values which will lead to inconsistency behavior.
If your configuration file includes IP addresses (node address, sans, etcd peerAddress), remember to update them accordingly for this specific controller node.

```shell
sudo k0s start
```

### 6. Check k0s status

To get general information about your k0s instance's status:

```shell
 sudo k0s status
```

```shell
Version: v{{{ extra.k8s_version }}}+k0s.0
Process ID: 2769
Parent Process ID: 1
Role: controller
Init System: linux-systemd
Service file: /etc/systemd/system/k0scontroller.service
```

### 7. Access your cluster

Use the Kubernetes 'kubectl' command-line tool that comes with k0s binary to deploy your application or check your node status:

```shell
sudo k0s kubectl get nodes
```

```shell
NAME   STATUS   ROLES    AGE    VERSION
k0s    Ready    <none>   4m6s   v{{{ extra.k8s_version }}}+k0s
```

You can also access your cluster easily with [Lens](https://k8slens.dev/), simply by copying the kubeconfig and pasting it to Lens:

```shell
sudo cat /var/lib/k0s/pki/admin.conf
```

**Note**: To access the cluster from an external network you must replace `localhost` in the kubeconfig with the host ip address for your controller.

## Next Steps

- [Install using k0sctl](k0sctl-install.md): Deploy multi-node clusters using just one command
- [Control plane configuration options](configuration.md): Networking and datastore configuration
- [Worker node configuration options](worker-node-config.md): Node labels and kubelet arguments
- [Support for cloud providers](cloud-providers.md): Load balancer or storage configuration
- [Installing the Traefik Ingress Controller](examples/traefik-ingress.md): Ingress deployment information
