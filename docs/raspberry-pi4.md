# Creating Raspberry Pi 4 Cluster

This is a highly opinionated example of deploying the [K0s](https://github.com/k0sproject/k0s) distribution of [Kubernetes](https://kubernetes.io) to a cluster comprised of [Raspberry Pi 4 Computers](https://www.raspberrypi.org/products/raspberry-pi-4-model-b/) with [Ubuntu 20.04 LTS](https://ubuntu.com) as the operating system.

## Prerequisites

The following tools should be installed on your local workstation to use this example:

* [Kubectl](https://kubernetes.io/docs/tasks/tools/install-kubectl/) `v1.19.4`+
* [Raspberry Pi Imager](https://github.com/raspberrypi/rpi-imager) `v1.5`+

## Walkthrough

In order to deploy K0s on your Raspberry Pi systems we'll follow these steps:

1. Hardware & Operating System Setup
2. Networking Configurations
3. Node Configurations
4. Deploying Kubernetes

These steps require a fair amount of pre-requisite knowledge of [Linux](https://linux.com) and assume a basic understanding of the [Ubuntu Linux Distribution](https://ubuntu.com) as well as [Kubernetes](https://kubernetes.io).

If you're feeling out of sorts, consider reading through the [Kubernetes Basics Documentation](https://kubernetes.io/docs/tutorials/kubernetes-basics/) for more context and some less complex exercises to get started with.

## Hardware & Operating System

**Note** that this example was developed with [Raspberry Pi 4 Model B Computers](https://www.raspberrypi.org/products/raspberry-pi-4-model-b/) with 8GB of RAM and 64GB SD Cards. You may have to do some manual editing of the example code and k0s configuration to get things working in your environment if you use lower spec machines.

### Downloading Ubuntu

When this example was developed the following image was used:

* [Ubuntu Server 20.04.1 LTS RASPI 4 Image](https://ubuntu.com/download/raspberry-pi/thank-you?version=20.04.1&architecture=server-arm64+raspi)

It's likely later versions and other versions will work but when in doubt consider using the above image for the nodes.

### Installing Ubuntu

The [Raspberry Pi Foundation](https://raspberrypi.org) provides a convenient [imaging tool](https://www.raspberrypi.org/blog/raspberry-pi-imager-imaging-utility/) for writing the Ubuntu image to your nodes' SD cards by selecting the image and the SD card you want to write it to.

If all goes well it should be as simple as flashing the SD, inserting it into the Raspberry Pi system, and turning it on and Ubuntu will bootstrap itself. If you run into any trouble however, Ubuntu provides [documentation on installing Ubuntu on Raspberry Pi Computers](https://ubuntu.com/tutorials/how-to-install-ubuntu-on-your-raspberry-pi).

Note that the default login credentials for the systems once [cloud-init](https://cloud-init.io/) finishes bootstrapping the system will be user `ubuntu` with password `ubuntu` and you'll be required to change the password on first login.

## Networking

For this example it's assumed that you have all computers connected to eachother on the same subnet, but the rest is pretty much open ended.

Make sure that you review the [K0s required ports documentation](https://github.com/k0sproject/k0s/blob/main/docs/networking.md#needed-open-ports--protocols) to ensure that your network and firewall configurations will allow necessary traffic for the cluster.

Review the [Ubuntu Server Networking Configuration Documentation](https://ubuntu.com/server/docs/network-configuration) and ensure that all systems get a static IP address on the network, or that the network is providing a static DHCP lease for the nodes.

### OpenSSH

Ubuntu Server will deploy and enable [OpenSSH](https://www.openssh.com/) by default, but make sure that for whichever user you're going to deploy the cluster with on the build system their [SSH Key is copied to each node's root user](https://www.cyberciti.biz/faq/use-ssh-copy-id-with-an-openssh-server-listing-on-a-different-port/), or you may have to do additional manual configurations to run the example.

Effectively before you start, you should have it configured so that the current user can run:

```shell
$ ssh root@${HOST}
```

Where `${HOST}` is any node and the login will succeed with no further prompts.

## Setup Nodes

Each node (whether control plane or not) will need some additional setup to prepare for K0s deployment.

### CGroup Configuration

Ensure that the following packages are installed on each node:

```shell
$ apt-get install cgroup-lite cgroup-tools cgroupfs-mount
```

Additionally not all Ubuntu images are going to have the `memory` cgroup enabled in the Kernel by default, one simple way to enable this is by adding it to the Kernel command line.

Open the file `/boot/firmware/cmdline.txt` which is responsible for managing the Kernel parameters, and ensure that the following parameters exist (add them if not):

```shell
cgroup_enable=cpuset cgroup_enable=memory cgroup_memory=1
```

Make sure you `reboot` each node to ensure the `memory` cgroup is loaded.

### Swap (Optional)

While this is _technical optional_ if you don't have the 8GB RAM Raspberry PI for your nodes and instead have the 4GB it can be helpful to enable swap to ease some memory pressure.

You can create a swapfile by running the following:

```shell
fallocate -l 2G /swapfile
chmod 0600 /swapfile
mkswap /swapfile
swapon -a
```

To ensure that the usage of swap is not too agressive, make sure you set the `sudo sysctl vm.swappiness=10` (the default is generally higher). Configure this in `/etc/sysctl.d/*` to be persistent.

Lastly to ensure that your swap is mounted after reboots, make sure the following line exists in your `/etc/fstab` configuration:

```shell
/swapfile         none           swap sw       0 0
```

### Kernel Modules

Some important Kernel modules to keep track of are the `overlay`, `nf_conntrack` and `br_netfilter` modules, ensure those are loaded:

```shell
$ modprobe overlay
$ modprobe nf_conntrack
$ modprobe br_netfilter
```

Add each of these modules to your `/etc/modules-load.d/modules.conf` file as well to ensure they persist after reboot.

### Download K0s

Download a [K0s release](https://github.com/k0sproject/k0s/releases/tag/v0.9.1), for example:

```shell
$ wget -O /tmp/k0s https://github.com/k0sproject/k0s/releases/download/v0.9.1/k0s-v0.9.1-arm64
$ chmod a+x /tmp/k0s
$ sudo mv /tmp/k0s /usr/bin/k0s
```

Now you'll be able to run `k0s`:

```shell
$ k0s version
v0.9.1
```

## Deploying Kubernetes

Each node is now setup to handle being a control plane node or worker node.

### Control Plane Node

For this demonstration, we'll use a non-ha control plane with a single node.

#### Systemd Service

Create a systemd service file for `k0s`:

```shell
$ cat << EOF > /etc/systemd/system/k0s.service
[Unit]
Description=k0s - Kubernetes Control Plane & Worker
ConditionFileIsExecutable=/usr/bin/k0s
After=network.target

[Service]
EnvironmentFile=/etc/sysconfig/k0s
ExecStart=/usr/bin/k0s controller
KillMode=process
Restart=always
RestartSec=120
StartLimitBurst=10
StartLimitInterval=5

[Install]
WantedBy=multi-user.target
EOF
```

Enable and start the service:

```shell
$ systemctl enable --now k0s
```

Run `systemctl status k0s` to verify the service status.

#### Worker Tokens

For each worker node that you expect to have, create a join token (and save this for later steps):

```shell
$ k0s token create --role worker
```

### Worker

For any number of worker nodes which you created join tokens for we'll need to deploy a worker service and start it.

#### Systemd Service

Create the join token for the worker:

```shell
$ mkdir -p /var/lib/k0s/
$ echo ${TOKEN_CONTENT} > /var/lib/k0s/join-token
```

Where `${TOKEN_CONTENT}` is one of the join tokens you created in the control plane setup.

Then deploy the systemd service for the worker:

```shell
$ cat << EOF > /etc/systemd/system/k0s.service
[Unit]
Description=k0s - Kubernetes Worker
ConditionFileIsExecutable=/usr/bin/k0s
After=network.target

[Service]
EnvironmentFile=-/etc/sysconfig/k0s
ExecStart=/usr/bin/k0s worker --token-file /var/lib/k0s/join-token
KillMode=process
Restart=always
RestartSec=120
StartLimitBurst=10
StartLimitInterval=5

[Install]
WantedBy=multi-user.target
EOF
```

Enable and start the service:

```shell
$ systemctl enable --now k0s
```

Run `systemctl status k0s` to verify the service status.

## Connecting To Your Cluster

Now generate a `kubeconfig` for the cluster and start managing it with `kubectl`:

```shell
ssh root@${CONTROL_PLANE_NODE} k0s kubeconfig create --groups "system:masters" k0s > config.yaml
export KUBECONFIG=$(pwd)/config.yaml
kubectl create clusterrolebinding k0s-admin-binding --clusterrole=admin --user=k0s
```

Where `${CONTROL_PLANE_NODE}` is the address of your control plane node.

Now the cluster can be accessed and used:

```shell
$ kubectl get nodes,deployments,pods -A
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

Enjoy!

