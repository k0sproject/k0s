# Install using k0sctl

k0sctl is a command-line tool for bootstrapping and managing k0s clusters. k0sctl connects to the provided hosts using SSH and gathers information on the hosts, with which it forms a cluster by configuring the hosts, deploying k0s, and then connecting the k0s nodes together.

![k0sctl deployment](img/k0sctl_deployment.png)

With k0sctl, you can create multi-node clusters in a manner that is automatic and easily repeatable. This method is recommended for production cluster installation.

**Note**: The k0sctl install method is necessary for automatic upgrade.

## Prerequisites

You can execute k0sctl on any system that supports the Go language. Pre-compiled k0sctl binaries are available on the [k0sctl releases page](https://github.com/k0sproject/k0sctl)).

**Note**: For target host prerequisites information, refer to the [k0s System Requirements](system-requirements.md).

## Install k0s

### 1. Install k0sctl tool

k0sctl is a single binary, the instructions for downloading and installing of which are available in the [k0sctl github repository](https://github.com/k0sproject/k0sctl#installation).

### 2. Configure the cluster

1. Run the following command to create a k0sctl configuration file:

    ```shell
    k0sctl init > k0sctl.yaml
    ```

    This action creates a `k0sctl.yaml` file in the current directory:

    ```yaml
    apiVersion: k0sctl.k0sproject.io/v1beta1
    kind: Cluster
    metadata:
      name: k0s-cluster
    spec:
      hosts:
      - role: controller
        ssh:
          address: 10.0.0.1 # replace with the controller's IP address
          user: root
          keyPath: ~/.ssh/id_rsa
      - role: worker
        ssh:
          address: 10.0.0.2 # replace with the worker's IP address
          user: root
          keyPath: ~/.ssh/id_rsa
    ```

2. Provide each host with a valid IP address that is reachable by k0sctl, and the connection details for an SSH connection.

 **Note**: Refer to the [k0sctl documentation](https://github.com/k0sproject/k0sctl#configuration-file-spec-fields) for k0sctl configuration specifications.

### 3. Deploy the cluster

Run `k0sctl apply` to perform the cluster deployment:

```shell
k0sctl apply --config k0sctl.yaml
```

```shell
⠀⣿⣿⡇⠀⠀⢀⣴⣾⣿⠟⠁⢸⣿⣿⣿⣿⣿⣿⣿⡿⠛⠁⠀⢸⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⠀█████████ █████████ ███
⠀⣿⣿⡇⣠⣶⣿⡿⠋⠀⠀⠀⢸⣿⡇⠀⠀⠀⣠⠀⠀⢀⣠⡆⢸⣿⣿⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀███          ███    ███
⠀⣿⣿⣿⣿⣟⠋⠀⠀⠀⠀⠀⢸⣿⡇⠀⢰⣾⣿⠀⠀⣿⣿⡇⢸⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⠀███          ███    ███
⠀⣿⣿⡏⠻⣿⣷⣤⡀⠀⠀⠀⠸⠛⠁⠀⠸⠋⠁⠀⠀⣿⣿⡇⠈⠉⠉⠉⠉⠉⠉⠉⠉⢹⣿⣿⠀███          ███    ███
⠀⣿⣿⡇⠀⠀⠙⢿⣿⣦⣀⠀⠀⠀⣠⣶⣶⣶⣶⣶⣶⣿⣿⡇⢰⣶⣶⣶⣶⣶⣶⣶⣶⣾⣿⣿⠀█████████    ███    ██████████

INFO k0sctl 0.0.0 Copyright 2021, Mirantis Inc.
INFO Anonymized telemetry will be sent to Mirantis.
INFO By continuing to use k0sctl you agree to these terms:
INFO https://k0sproject.io/licenses/eula
INFO ==> Running phase: Connect to hosts
INFO [ssh] 10.0.0.1:22: connected
INFO [ssh] 10.0.0.2:22: connected
INFO ==> Running phase: Detect host operating systems
INFO [ssh] 10.0.0.1:22: is running Ubuntu 20.10
INFO [ssh] 10.0.0.2:22: is running Ubuntu 20.10
INFO ==> Running phase: Prepare hosts
INFO [ssh] 10.0.0.1:22: installing kubectl
INFO ==> Running phase: Gather host facts
INFO [ssh] 10.0.0.1:22: discovered 10.12.18.133 as private address
INFO ==> Running phase: Validate hosts
INFO ==> Running phase: Gather k0s facts
INFO ==> Running phase: Download K0s on the hosts
INFO [ssh] 10.0.0.2:22: downloading k0s 0.11.0
INFO [ssh] 10.0.0.1:22: downloading k0s 0.11.0
INFO ==> Running phase: Configure K0s
WARN [ssh] 10.0.0.1:22: generating default configuration
INFO [ssh] 10.0.0.1:22: validating configuration
INFO [ssh] 10.0.0.1:22: configuration was changed
INFO ==> Running phase: Initialize K0s Cluster
INFO [ssh] 10.0.0.1:22: installing k0s controller
INFO [ssh] 10.0.0.1:22: waiting for the k0s service to start
INFO [ssh] 10.0.0.1:22: waiting for kubernetes api to respond
INFO ==> Running phase: Install workers
INFO [ssh] 10.0.0.1:22: generating token
INFO [ssh] 10.0.0.2:22: writing join token
INFO [ssh] 10.0.0.2:22: installing k0s worker
INFO [ssh] 10.0.0.2:22: starting service
INFO [ssh] 10.0.0.2:22: waiting for node to become ready
INFO ==> Running phase: Disconnect from hosts
INFO ==> Finished in 2m2s
INFO k0s cluster version 0.11.0 is now installed
INFO Tip: To access the cluster you can now fetch the admin kubeconfig using:
INFO      k0sctl kubeconfig
```

### 4. Access the cluster

To access your k0s cluster, use k0sctl to generate a `kubeconfig` for the purpose.

```shell
k0sctl kubeconfig > kubeconfig
```

With the `kubeconfig`, you can access your cluster using either kubectl or [Lens](https://k8slens.dev/).

```shell
kubectl get pods --kubeconfig kubeconfig -A
```

```shell
NAMESPACE     NAME                                       READY   STATUS    RESTARTS   AGE
kube-system   calico-kube-controllers-5f6546844f-w8x27   1/1     Running   0          3m50s
kube-system   calico-node-vd7lx                          1/1     Running   0          3m44s
kube-system   coredns-5c98d7d4d8-tmrwv                   1/1     Running   0          4m10s
kube-system   konnectivity-agent-d9xv2                   1/1     Running   0          3m31s
kube-system   kube-proxy-xp9r9                           1/1     Running   0          4m4s
kube-system   metrics-server-6fbcd86f7b-5frtn            1/1     Running   0          3m51s
```

## Known limitations

* k0sctl does not perform any discovery of hosts, and thus it only operates on the hosts listed in the provided configuration.
* k0sctl can only add more nodes to the cluster. It cannot remove existing nodes.

## Next Steps

* [Control plane configuration options](configuration.md): Networking and datastore configuration
* [Worker node configuration options](worker-node-config.md): Node labels and kubelet arguments
* [Support for cloud providers](cloud-providers.md): Load balancer or storage configuration
* [Installing the Traefik Ingress Controller](examples/traefik-ingress.md): Ingress deployment information
