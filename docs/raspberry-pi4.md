# Create a Raspberry Pi 4 Cluster

You can deploy the [k0s](https://github.com/k0sproject/k0s) distribution of [Kubernetes](https://kubernetes.io) to a cluster comprised of [Raspberry Pi 4 Computers](https://www.raspberrypi.org/products/raspberry-pi-4-model-b/) with [Ubuntu 20.04 LTS](https://ubuntu.com) as the operating system.

## Prerequisites

Install the following tools on your local system:

* [Kubectl](https://kubernetes.io/docs/tasks/tools/install-kubectl/) `v1.19.4`+
* [Raspberry Pi Imager](https://github.com/raspberrypi/rpi-imager) `v1.5`+
* [Raspberry Pi 4 Model B Computers](https://www.raspberrypi.org/products/raspberry-pi-4-model-b/) with 8GB of RAM and 64GB SD Cards.

**Note:** If you use lower spec Raspberry Pi machines, it may be necessary to manually edit the example code and k0s configuration.

## Install k0s

### Set up hardware and operating system

Download and install the [Ubuntu Server 20.04.1 LTS RASPI 4 Image](https://ubuntu.com/download/raspberry-pi/thank-you?version=20.04.1&architecture=server-arm64+raspi).

**Note**: In addition to the documentation Ubuntu provides [documentation on installing Ubuntu on Raspberry Pi Computers](https://ubuntu.com/tutorials/how-to-install-ubuntu-on-your-raspberry-pi), the [Raspberry Pi Foundation](https://raspberrypi.org) offers an[imaging tool](https://www.raspberrypi.org/blog/raspberry-pi-imager-imaging-utility/) that you can use to write the Ubuntu image to your noe SD cards.

Once [cloud-init](https://cloud-init.io/) finishes bootstrapping the system, the default login credentials are set to user `ubuntu` with password `ubuntu` (which you will be prompted to change on first login).

### Network configurations

**Note**: For network configurtion purposes, this documentation assumes that all of your computers are connected on the same subnet.

Review the [k0s required ports documentation](https://github.com/k0sproject/k0s/blob/main/docs/networking.md#needed-open-ports--protocols) to ensure that your network and firewall configurations allow necessary traffic for the cluster.

Review the [Ubuntu Server Networking Configuration Documentation](https://ubuntu.com/server/docs/network-configuration) to ensure that all systems have a static IP address on the network, or that the network is providing a static DHCP lease for the nodes.

#### OpenSSH

Ubuntu Server deploys and enables [OpenSSH](https://www.openssh.com/) by default. Confirm, though, that for whichever user you will deploy the cluster with on the build system, their [SSH Key is copied to each node's root user](https://www.cyberciti.biz/faq/use-ssh-copy-id-with-an-openssh-server-listing-on-a-different-port/). Before you start, the configuration should be such that the current user can run:

```shell
ssh root@${HOST}
```

Where `${HOST}` is any node and the login can succeed with no further prompts.

### Set up Nodes

Every node (whether control plane or not) requires additional configuration in preparation for k0s deployment.

#### CGroup Configuration

1. Ensure that the following packages are installed on each node:

    ```shell
    apt-get install cgroup-lite cgroup-tools cgroupfs-mount
    ```

2. Enable the `memory` cgroup in the Kernel by adding it to the Kernel command line.

3. Open the file `/boot/firmware/cmdline.txt` (responsible for managing the Kernel parameters), and confirm that the following parameters exist (and add them as necessary):

    ```shell
    cgroup_enable=cpuset cgroup_enable=memory cgroup_memory=1
    ```

4. Be sure to reboot each node to ensure the `memory` cgroup is loaded.

#### Swap (Optional)

While swap is _technically optional_, enable it to ease memory pressure.

1. To create a swapfile:

    ```shell
    fallocate -l 2G /swapfile && \
    chmod 0600 /swapfile && \
    mkswap /swapfile && \
    swapon -a
    ```

2. Ensure that the usage of swap is not too agressive by setting the `sudo sysctl vm.swappiness=10` (the default is generally higher) and configuring it to be persistent in `/etc/sysctl.d/*`.

3. Ensure that your swap is mounted after reboots by confirming that the following line exists in your `/etc/fstab` configuration:

    ```shell
    /swapfile         none           swap sw       0 0
    ```

#### Kernel Modules

Ensure the loading of the `overlay`, `nf_conntrack` and `br_netfilter` modules:

```shell
modprobe overlay
modprobe nf_conntrack
modprobe br_netfilter
```

In addition, add each of these modules to your `/etc/modules-load.d/modules.conf` file to ensure they persist following reboot.

#### Download k0s

Download a [k0s release](https://github.com/k0sproject/k0s/releases/latest). For example:

```shell
wget -O /tmp/k0s https://github.com/k0sproject/k0s/releases/download/v0.9.1/k0s-v0.9.1-arm64 # replace version number!
sudo install /tmp/k0s /usr/local/bin/k0s
```

-- or --

Use the k0s download script (as one command) to download the latest stable
k0s and make it executable in `/usr/bin/k0s`.

```shell
curl -sSLf https://get.k0s.sh | sudo sh
```

At this point you can run `k0s`:

```shell
k0s version
```

```shell
v1.22.1+k0s.0
```

### Deploy Kubernetes

Each node can now serve as a control plane node or worker node.

#### Control Plane Node

Use a non-ha control plane with a single node.

##### Systemd Service (controller)

1. Create a systemd service:

    ```shell
    sudo k0s install controller
    ```

2. Start the service:

    ```shell
    sudo k0s start
    ```

3. Run `sudo k0s status` or `systemctl status k0scontroller` to verify the service status.

##### Worker Tokens

For each worker node that you expect to have, create a join token:

```shell
k0s token create --role worker
```

Save the join token for subsequent steps.

#### Worker

You must deploy and start a worker service for each worker nodes for which you created join tokens.

##### Systemd Service (worker)

1. Create the join token file for the worker (where `TOKEN_CONTENT` is one of the join tokens created in the control plane setup):

    ```shell
    mkdir -p /var/lib/k0s/
    echo TOKEN_CONTENT > /var/lib/k0s/join-token
    ```

2. Deploy the systemd service for the worker:

    ```shell
    sudo k0s install worker --token-file /var/lib/k0s/join-token
    ```

3. Start the service:

    ```shell
    sudo k0s start
    ```

4. Run `sudo k0s status` or `systemctl status k0sworker` to verify the service status.

### Connect To Your Cluster

Generate a `kubeconfig` for the cluster and begin managing it with `kubectl` (where `CONTROL_PLANE_NODE` is the control plane node address):

```shell
ssh root@CONTROL_PLANE_NODE k0s kubeconfig create --groups "system:masters" k0s > config.yaml
export KUBECONFIG=$(pwd)/config.yaml
kubectl create clusterrolebinding k0s-admin-binding --clusterrole=admin --user=k0s
```

You can now access and use the cluster:

```shell
kubectl get nodes,deployments,pods -A
```

```shell
NAME         STATUS   ROLES    AGE     VERSION
node/k8s-4   Ready    <none>   5m9s    v1.20.1-k0s1
node/k8s-5   Ready    <none>   5m      v1.20.1-k0s1
node/k8s-6   Ready    <none>   4m45s   v1.20.1-k0s1

NAMESPACE     NAME                                      READY   UP-TO-DATE   AVAILABLE   AGE
kube-system   deployment.apps/calico-kube-controllers   1/1     1            1           12m
kube-system   deployment.apps/coredns                   1/1     1            1           12m

NAMESPACE     NAME                                           READY   STATUS        RESTARTS   AGE
kube-system   pod/calico-kube-controllers-5f6546844f-rjdkz   1/1     Running       0          12m
kube-system   pod/calico-node-j475n                          1/1     Running       0          5m9s
kube-system   pod/calico-node-lnfrf                          1/1     Running       0          4m45s
kube-system   pod/calico-node-pzp7x                          1/1     Running       0          5m
kube-system   pod/coredns-5c98d7d4d8-bg9pl                   1/1     Running       0          12m
kube-system   pod/konnectivity-agent-548hp                   1/1     Running       0          4m45s
kube-system   pod/konnectivity-agent-66cr8                   1/1     Running       0          4m49s
kube-system   pod/konnectivity-agent-lxt9z                   1/1     Running       0          4m58s
kube-system   pod/kube-proxy-ct6bg                           1/1     Running       0          5m
kube-system   pod/kube-proxy-hg8t2                           1/1     Running       0          4m45s
kube-system   pod/kube-proxy-vghs9                           1/1     Running       0          5m9s
```
