<!--
SPDX-FileCopyrightText: 2020 k0s authors
SPDX-License-Identifier: CC-BY-SA-4.0
-->

# Quick Start Guide

Completing this Quick Start results in a single-node Kubernetes cluster that
incorporates both the controller and worker roles. This setup is ideal for
environments that don't require high availability or multiple nodes.

## Prerequisites

**Note**: Before proceeding, make sure to review the [System Requirements](system-requirements.md).

The following steps work on every typical Linux distribution that uses either
systemd or OpenRC as its init system.

## Install k0s

1. Download k0s

    Run the k0s download script to download the latest stable version of k0s and make it executable from /usr/local/bin/k0s.

    ```shell
    curl --proto '=https' --tlsv1.2 -sSf https://get.k0s.sh | sudo sh
    ```

    Alternatively, download it from the [k0s releases page]. This approach is
    required in air gapped environments.

2. Install k0s as a service

    The `k0s install` sub-command installs k0s as a system service on a host that is running one of the supported init systems: systemd or OpenRC. You can execute the install for workers, controllers or single node (controller+worker) instances.

    Run the following command to install a single node k0s that includes the controller and worker functions with the default configuration:

    ```shell
    sudo k0s install controller --single
    ```

    **Note**: The `--single` option disables features needed for multi-node
    clusters, so the cluster cannot be extended. To retain the ability to expand
    the cluster in the future, use:

    ``` shell
    sudo k0s install controller --enable-worker --no-taints
    ```

    The `k0s install controller` sub-command accepts the same flags and parameters as the `k0s controller`. Refer to [manual install](k0s-multi-node.md#install-k0s) for a custom config file example.

    It is possible to set environment variables with the install command:

    ```shell
    sudo k0s install controller -e ETCD_UNSUPPORTED_ARCH=arm
    ```

    The system service can be reinstalled with the `--force` flag:

    ```shell
    sudo k0s install controller --single --force
    sudo systemctl daemon-reload
    ```

3. Start k0s as a service

    To start the k0s service, run:

    ```shell
    sudo k0s start
    ```

    The k0s service will start automatically after the node restart.

    A minute or two typically passes before the node is ready to deploy applications.

4. Check service, logs and k0s status

    Run the following command to check the k0s instance status:

    ```shell
    $ sudo k0s status
    Version: {{{ k0s_version }}}
    Process ID: 436
    Role: controller
    Workloads: true
    Init System: linux-systemd
    ```

5. Access the cluster using kubectl

    **Note**: k0s embeds the Kubernetes command-line tool *kubectl*.

    Use kubectl to deploy applications or check node status:

    ```shell
    $ sudo k0s kubectl get nodes
    NAME   STATUS   ROLES    AGE    VERSION
    k0s    Ready    <none>   4m6s   {{{ k8s_version }}}+k0s
    ```

[k0s releases page]: https://github.com/k0sproject/k0s/releases/latest

## Uninstall k0s

The removal of k0s is a two-step process.

1. Stop the service.

    ```shell
    sudo k0s stop
    ```

2. Execute the `k0s reset` command.

     The `k0s reset` command cleans up the installed system service, data directories, containers, mounts and network namespaces.

    ```shell
    sudo k0s reset
    ```

3. Reboot the system.

    A few small k0s fragments persist even after the reset, such as iptables
    rules. Reboot the machine after running `k0s reset`.

## Next Steps

- [Install using k0sctl](k0sctl-install.md): Deploy multi-node clusters using just one command
- [Manual Install](k0s-multi-node.md): (Advanced) Manually deploy multi-node clusters
- [Control plane configuration options](configuration.md): Networking and data store configuration
- [Worker node configuration options](worker-node-config.md): Node labels and kubelet arguments
- [Support for cloud providers](cloud-providers.md): Load balancer or storage configuration
- [Installing the Traefik Ingress Controller](examples/traefik-ingress.md):
  Ingress deployment information
- [Air gapped/offline installation](airgap-install.md): Air gaped deployment
