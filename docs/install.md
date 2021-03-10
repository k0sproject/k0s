# Getting Started

 In this tutorial you'll create a full Kubernetes cluster with just one node including both the controller and the worker. This is well suited for environments where the high-availability and multiple nodes are not needed. This is the easiest install method to start experimenting k0s.

### Prerequisites

This tutorial has been written for Debian/Ubuntu, but it can be used for any Linux running one of the supported init systems: Systemd or OpenRC.

Before proceeding, make sure to review the [System Requirements](system-requirements.md).

### Installation steps

#### 1. Download k0s

The k0s download script downloads the latest stable k0s and makes it executable from /usr/bin/k0s.
```sh
$ curl -sSLf https://get.k0s.sh | sudo sh
```

#### 2. Install k0s as a service

The `k0s install` sub-command will install k0s as a system service on the local host running one of the supported init systems: Systemd or OpenRC. Install can be executed for workers, controllers or single node (controller+worker) instances.

This command will install a single node k0s including the controller and worker functions with the default configuration:

```sh
$ sudo k0s install controller --enable-worker
```

The `k0s install controller` sub-command accepts the same flags and parameters as the `k0s controller`. See [manual install](k0s-multi-node.md#installation-steps) for an example for entering a custom config file.

#### 3. Start k0s as a service

To start the k0s service, run
```sh
$ sudo systemctl start k0scontroller
```
It usually takes 1-2 minutes until the node is ready for deploying applications.

If you want to enable the k0s service to be started always after the node restart, enable the service. This command is optional. 
```sh
$ sudo systemctl enable k0scontroller
```

#### 4. Check service, logs and k0s status

You can check the service status and logs like this:
```sh
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
```sh
$ sudo k0s status
Version: v0.11.0
Process ID: 436
Parent Process ID: 1
Role: controller+worker
Init System: linux-systemd
```

#### 5. Access your cluster using kubectl

The Kubernetes command-line tool 'kubectl' is included into k0s. You can use it for example to deploy your application or check your node status like this:
```sh
$ sudo k0s kubectl get nodes
NAME   STATUS   ROLES    AGE    VERSION
k0s    Ready    <none>   4m6s   v1.20.4-k0s1
```

#### 6. Clean-up

If you want to remove the k0s installation you should first stop the service:
```sh
$ sudo systemctl stop k0scontroller
```

Then you can execute `k0s reset`, which cleans up the installed system service, data directories, containers, mounts and network namespaces. There are still few bits (e.g. iptables) that cannot be easily cleaned up and thus a reboot after the reset is highly recommended.
```sh
$ sudo k0s reset
```

### Next Steps

- [Installing with k0sctl](k0sctl-install.md) for deploying and upgrading multi-node clusters with one command
- [Manual Install](k0s-multi-node.md) for advanced users for manually deploying multi-node clusters
- [Control plane configuration options](configuration.md) for example for networking and datastore configuration
- [Worker node configuration options](worker-node-config.md) for example for node labels and kubelet arguments
- [Support for cloud providers](cloud-providers.md) for example for load balancer or storage configuration
- [Installing the Traefik Ingress Controller](examples/traefik-ingress.md), a tutorial for ingress deployment
- [Airgap/Offline installation](airgap-install.md), a tutorial for airgap deployment