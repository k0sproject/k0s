# Dockershim Deprecation - What Does It Mean For K0s?

Back in December 2020, Kubernetes have announced the [deprecation of the
docker-shim][deprecate-dockershim] from version 1.24 onwards. As a consequence,
k0s 1.24 and above don't support the docker-shim as well.

[deprecate-dockershim]: https://kubernetes.io/blog/2020/12/02/dockershim-faq/

## What Is Dockershim, and Why Was It Deprecated?

The dockershim is a transparent library that intercepts API calls to the kubernetes API and handles their operation in the Docker API. Early versions of Kubernetes used this shim in order to allow containers to run over docker. Later versions of Kubernetes started creating containers via the CRI (Container Runtime Interface). Since CRI has become the de-facto default runtime for Kubernetes, maintaining the dockershim turned into a heavy burden for Kubernetes maintainers, and so the decision to deprecate the built-in dockershim support came into being.

### So What's going to happen to Dockershim?

Dockershim is not gone. It's only changed ownership. Mirantis has agreed to maintain dockershim (now called cri-dockerd). See: [The Future of Dockershim is cri-dockerd](https://www.mirantis.com/blog/the-future-of-dockershim-is-cri-dockerd/).

From Kubernetes version 1.24 you will have the built-in possibility to run containers via CRI, but if you want to continue using docker, you are free to do so, using [cri-dockerd](https://github.com/Mirantis/cri-dockerd).

In order to continue to use the Docker engine with Kubernetes v1.24+, you will have to migrated all worker nodes to use cri-dockerd.

## Migrating to CRI-Dockerd

*This migration guide assumes that you've been running k0s with docker on version 1.23 and below.*

The following steps will need to be done on ALL k0s' worker nodes, or single-node controllers. Basically any node that runs containers will need to be migrated using the process detailed below.

### Cordon and drain the node

Get a list of all nodes (k0s is still version 1.23, which already includes the docker-shim):

```sh
sudo k0s kubectl get nodes -o wide

NAME                                        STATUS   ROLES           AGE   VERSION       INTERNAL-IP   EXTERNAL-IP   OS-IMAGE             KERNEL-VERSION    CONTAINER-RUNTIME
ip-10-0-49-188.eu-west-1.compute.internal   Ready    control-plane   52m   v{{{ extra.k8s_version }}}+k0s   10.0.49.188   <none>        Ubuntu 20.04.4 LTS   5.13.0-1022-aws   docker://20.10.16
ip-10-0-62-250.eu-west-1.compute.internal   Ready    <none>          12s   v{{{ extra.k8s_version }}}+k0s   10.0.62.250   <none>        Ubuntu 20.04.4 LTS   5.13.0-1017-aws   docker://20.10.16
```

cordon and drain the nodes (migrate one by one):

```sh
sudo k0s kubectl cordon ip-10-0-62-250.eu-west-1.compute.internal 
sudo k0s kubectl drain ip-10-0-62-250.eu-west-1.compute.internal --ignore-daemonsets
```

```sh
sudo k0s kubectl get nodes -o wide

NAME                                        STATUS                     ROLES           AGE     VERSION       INTERNAL-IP   EXTERNAL-IP   OS-IMAGE             KERNEL-VERSION    CONTAINER-RUNTIME
ip-10-0-49-188.eu-west-1.compute.internal   Ready                      control-plane   56m     v{{{ extra.k8s_version }}}+k0s   10.0.49.188   <none>        Ubuntu 20.04.4 LTS   5.13.0-1022-aws   docker://20.10.16
ip-10-0-62-250.eu-west-1.compute.internal   Ready,SchedulingDisabled   <none>          3m40s   v{{{ extra.k8s_version }}}+k0s   10.0.62.250   <none>        Ubuntu 20.04.4 LTS   5.13.0-1017-aws   docker://20.10.16
```

Stop k0s on the node:

```sh
sudo k0s stop
```

### Installing CRI-Dockerd

Download the Latest cri-dockerd deb package:

```sh
cd /tmp

# Get the deb file name for ubuntu-jammy
OS="ubuntu-jammy"
PKG=$(curl -s https://api.github.com/repos/Mirantis/cri-dockerd/releases/latest | grep ${OS} | grep http | cut -d '"' -f 4)

wget ${PKG} -O cri-dockerd-latest.deb

sudo dpkg -i cri-dockerd-latest.deb

Selecting previously unselected package cri-dockerd.
(Reading database ... 164618 files and directories currently installed.)
Preparing to unpack cri-dockerd-latest.deb ...
Unpacking cri-dockerd (0.2.1~3-0~ubuntu-jammy) ...
Setting up cri-dockerd (0.2.1~3-0~ubuntu-jammy) ...
Created symlink /etc/systemd/system/multi-user.target.wants/cri-docker.service → /lib/systemd/system/cri-docker.service.
Created symlink /etc/systemd/system/sockets.target.wants/cri-docker.socket → /lib/systemd/system/cri-docker.socket.
```

Verify the correct version:

```sh
which cri-dockerd
/usr/bin/cri-dockerd

cri-dockerd --version
cri-dockerd 0.2.1 (HEAD)
```

Make sure dockershim is started:

```sh
sudo systemctl status cri-docker.service
● cri-docker.service - CRI Interface for Docker Application Container Engine
     Loaded: loaded (/lib/systemd/system/cri-docker.service; enabled; vendor preset: enabled)
     Active: active (running) since Wed 2022-05-25 14:27:31 UTC; 1min 23s ago
TriggeredBy: ● cri-docker.socket
       Docs: https://docs.mirantis.com
   Main PID: 1404151 (cri-dockerd)
      Tasks: 9
     Memory: 15.3M
     CGroup: /system.slice/cri-docker.service
             └─1404151 /usr/bin/cri-dockerd --container-runtime-endpoint fd:// --network-plugin=

```

### Configure K0s to use dockershim

Replace docker socket in the systemd file for cri-dockerd (the step below should be run AFTER upgrading k0s to version 1.24):

```sh
sudo sed -i -e 's_--cri-socket=docker:unix:///var/run/docker.sock_--cri-socket docker:unix:///var/run/cri-dockerd.sock_' /etc/systemd/system/k0sworker.service
sudo systemctl daemon-reload
```

### Start k0s with cri-dockerd

```sh
sudo k0s start
```

Verify the running pods via `docker ps`:

```sh
docker ps --format "table {{.ID}}\t{{.Names}}\t{{.State}}\t{{.Status}}\t{{.Image}}"

CONTAINER ID   NAMES                                                                                                STATE     STATUS          IMAGE
1b9b4624ddfd   k8s_konnectivity-agent_konnectivity-agent-5jpd7_kube-system_1b3101ea-baeb-4a22-99a2-088d7ca5be85_1   running   Up 51 minutes   quay.io/k0sproject/apiserver-network-proxy-agent
414758a8a951   k8s_kube-router_kube-router-qlkgg_kube-system_9a1b67bf-5347-4acd-98ac-f9a67f2db730_1                 running   Up 51 minutes   3a67679337a5
b81960bb304c   k8s_kube-proxy_kube-proxy-tv95n_kube-system_164dc9f8-f47c-4f6c-acb7-ede5dbcd63cd_1                   running   Up 51 minutes   registry.k8s.io/kube-proxy
fb888cbc5ae0   k8s_POD_kube-router-qlkgg_kube-system_9a1b67bf-5347-4acd-98ac-f9a67f2db730_0                         running   Up 51 minutes   registry.k8s.io/pause:3.1
382d0a938c9d   k8s_POD_konnectivity-agent-5jpd7_kube-system_1b3101ea-baeb-4a22-99a2-088d7ca5be85_0                  running   Up 51 minutes   registry.k8s.io/pause:3.1
72d4a47b5609   k8s_POD_kube-proxy-tv95n_kube-system_164dc9f8-f47c-4f6c-acb7-ede5dbcd63cd_0                          running   Up 51 minutes   registry.k8s.io/pause:3.1
```

On the controller, you'll be able to see the worker started with the new docker container runtime:

```sh
sudo k0s kubectl get nodes -o wide

NAME                                        STATUS                     ROLES           AGE    VERSION       INTERNAL-IP   EXTERNAL-IP   OS-IMAGE             KERNEL-VERSION    CONTAINER-RUNTIME
ip-10-0-49-188.eu-west-1.compute.internal   Ready                      control-plane   117m   v{{{ extra.k8s_version }}}+k0s   10.0.49.188   <none>        Ubuntu 20.04.4 LTS   5.13.0-1022-aws   docker://20.10.16
ip-10-0-62-250.eu-west-1.compute.internal   Ready,SchedulingDisabled   <none>          64m    v{{{ extra.k8s_version }}}+k0s   10.0.62.250   <none>        Ubuntu 20.04.4 LTS   5.13.0-1017-aws   docker://20.10.16
```

### Uncordon the Node

```sh
sudo k0s kubectl uncordon ip-10-0-62-250.eu-west-1.compute.internal

node/ip-10-0-62-250.eu-west-1.compute.internal uncordoned
```

You should now see the node Ready for scheduling with the docker Runtime:

```sh
sudo k0s kubectl get nodes -o wide

NAME                                        STATUS   ROLES           AGE    VERSION       INTERNAL-IP   EXTERNAL-IP   OS-IMAGE             KERNEL-VERSION    CONTAINER-RUNTIME
ip-10-0-49-188.eu-west-1.compute.internal   Ready    control-plane   119m   v{{{ extra.k8s_version }}}+k0s   10.0.49.188   <none>        Ubuntu 20.04.4 LTS   5.13.0-1022-aws   docker://20.10.16
ip-10-0-62-250.eu-west-1.compute.internal   Ready    <none>          66m    v{{{ extra.k8s_version }}}+k0s   10.0.62.250   <none>        Ubuntu 20.04.4 LTS   5.13.0-1017-aws   docker://20.10.16
```
