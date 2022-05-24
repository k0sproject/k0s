# Dockershim Deprecation - What Does It Mean For K0s?

Back in December 2020, Kubernetes have anounced the [deprecation of the docker-shim from version 1.24 onwards](https://kubernetes.io/blog/2020/12/02/dockershim-faq/). Now that kubernetes 1.24 is out, the 1.24 release of k0s will no longer support the docker-shim as well.

## What Is Dockershim, and Why Was It Deprecated?

The dockershim is a transparent library that intercepts API calls to the kubernetes API and handles their operation in the Docker API. Early versions of Kubernetes used this shim in order to allow containers to run over docker. Later versions of Kubernetes started creating containers via the CRI (Container Runtime Interface). Since CRI has become the de-facto default runtime for Kubernetes, maintaining the dockershin turned into a heavy burden for Kubernetes maintainers, and so the decision to deprecate the built-in dockershim support came into being.

### So What's going to happen to Dockershim?

Dockershim is not gone. It's only changed ownership. Mirantis has agreed to maintain dockershim (now called cri-dockerd). See: [The Future of Dockershim is cri-dockerd](https://www.mirantis.com/blog/the-future-of-dockershim-is-cri-dockerd/).

From Kubernetes version 1.24 you will have the built-in possibility to run containers via CRI, but if you want to continue using docker, you are free to do so, using [cri-dockerd](https://github.com/Mirantis/cri-dockerd).

In order to continue to use the Docker engine with Kubernetes v1.24+, you will have to migrated all worker nodes to use cri-dockerd.

## Migrating to CRI-Dockerd

The following steps will need to be done on ALL k0s' worker nodes, or single-nodes. Basically any node that runs containers will need to be migrated using the process detailed below.

### Cordon and drain the node

Get a list of all nodes:

```sh
sudo k0s kubectl get nodes -o wide

NAME                                        STATUS   ROLES           AGE   VERSION       INTERNAL-IP   EXTERNAL-IP   OS-IMAGE             KERNEL-VERSION    CONTAINER-RUNTIME
ip-10-0-49-188.eu-west-1.compute.internal   Ready    control-plane   3m    v1.24.0+k0s   10.0.49.188   <none>        Ubuntu 20.04.4 LTS   5.13.0-1022-aws   containerd://1.6.4
ip-10-0-62-250.eu-west-1.compute.internal   Ready    <none>          31s   v1.24.0+k0s   10.0.62.250   <none>        Ubuntu 20.04.4 LTS   5.13.0-1017-aws   containerd://1.6.4
```

cordon and drain the nodes (migrate one by one):

```sh
sudo k0s kubectl cordon ip-10-0-62-250.eu-west-1.compute.internal 
sudo k0s kubectl drain ip-10-0-62-250.eu-west-1.compute.internal --ignore-daemonsets
```

```sh
sudo k0s kubectl get nodes -o wide

NAME                                        STATUS                     ROLES           AGE     VERSION       INTERNAL-IP   EXTERNAL-IP   OS-IMAGE             KERNEL-VERSION    CONTAINER-RUNTIME
ip-10-0-49-188.eu-west-1.compute.internal   Ready                      control-plane   12m     v1.24.0+k0s   10.0.49.188   <none>        Ubuntu 20.04.4 LTS   5.13.0-1022-aws   containerd://1.6.4
ip-10-0-62-250.eu-west-1.compute.internal   Ready,SchedulingDisabled   <none>          9m47s   v1.24.0+k0s   10.0.62.250   <none>        Ubuntu 20.04.4 LTS   5.13.0-1017-aws   containerd://1.6.4
```

Stop k0s on the node:

```sh
sudo k0s stop
```

### Installing CRI-Dockerd

Download the Latest cri-dockerd version:

```sh
cd /tmp
VER=$(curl -s https://api.github.com/repos/Mirantis/cri-dockerd/releases/latest|grep tag_name | cut -d '"' -f 4)

wget https://github.com/Mirantis/cri-dockerd/releases/download/${VER}/cri-dockerd-${VER}-linux-amd64.tar.gz
tar xvf cri-dockerd-${VER}-linux-amd64.tar.gz

sudo mv cri-dockerd /usr/local/bin/
```

Verify the correct version:

```sh
cri-dockerd --version
cri-dockerd 0.2.0 (HEAD)
```

Download cri-dockerd packaging files:

```sh
wget https://raw.githubusercontent.com/Mirantis/cri-dockerd/master/packaging/systemd/cri-docker.service
wget https://raw.githubusercontent.com/Mirantis/cri-dockerd/master/packaging/systemd/cri-docker.socket
sudo mv cri-docker.socket cri-docker.service /etc/systemd/system/
sudo sed -i -e 's,/usr/bin/cri-dockerd,/usr/local/bin/cri-dockerd,' /etc/systemd/system/cri-docker.service
```

Enable dockershim:

```sh
sudo systemctl daemon-reload
sudo systemctl enable cri-docker.service
sudo systemctl enable --now cri-docker.socket
```

### Configure K0s to use dockershim

```sh
sudo sed -i -e 's_^ExecStart.*_& --cri-socket docker:unix:///var/run/cri-dockerd.sock_' /etc/systemd/system/k0sworker.service
sudo systemctl daemon-reload
```

This will add the cri-socket flag to the worker command:

```sh
ExecStart=/usr/local/bin/k0s worker --token-file=/home/ubuntu/worker_token.pem --cri-socket docker:unix:///var/run/cri-dockerd.sock
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
b81960bb304c   k8s_kube-proxy_kube-proxy-tv95n_kube-system_164dc9f8-f47c-4f6c-acb7-ede5dbcd63cd_1                   running   Up 51 minutes   k8s.gcr.io/kube-proxy
fb888cbc5ae0   k8s_POD_kube-router-qlkgg_kube-system_9a1b67bf-5347-4acd-98ac-f9a67f2db730_0                         running   Up 51 minutes   k8s.gcr.io/pause:3.1
382d0a938c9d   k8s_POD_konnectivity-agent-5jpd7_kube-system_1b3101ea-baeb-4a22-99a2-088d7ca5be85_0                  running   Up 51 minutes   k8s.gcr.io/pause:3.1
72d4a47b5609   k8s_POD_kube-proxy-tv95n_kube-system_164dc9f8-f47c-4f6c-acb7-ede5dbcd63cd_0                          running   Up 51 minutes   k8s.gcr.io/pause:3.1
```

On the controller, you'll be able to see the worker started with the new docker container runtime:

```sh
➜ sudo k0s kubectl get nodes -o wide
NAME                                        STATUS                     ROLES           AGE   VERSION       INTERNAL-IP   EXTERNAL-IP   OS-IMAGE             KERNEL-VERSION    CONTAINER-RUNTIME
ip-10-0-49-188.eu-west-1.compute.internal   Ready                      control-plane   16h   v1.24.0+k0s   10.0.49.188   <none>        Ubuntu 20.04.4 LTS   5.13.0-1022-aws   containerd://1.6.4
ip-10-0-62-250.eu-west-1.compute.internal   Ready,SchedulingDisabled   <none>          16h   v1.24.0+k0s   10.0.62.250   <none>        Ubuntu 20.04.4 LTS   5.13.0-1017-aws   docker://20.10.16
```

### Uncordon the Node

```sh
sudo k0s kubectl uncordon ip-10-0-62-250.eu-west-1.compute.internal

node/ip-10-0-62-250.eu-west-1.compute.internal uncordoned
```

You should now see the node Ready for scheduling with the docker Runtime:

```sh
sudo k0s kubectl get nodes -o wide

NAME                                        STATUS   ROLES           AGE   VERSION       INTERNAL-IP   EXTERNAL-IP   OS-IMAGE             KERNEL-VERSION    CONTAINER-RUNTIME
ip-10-0-49-188.eu-west-1.compute.internal   Ready    control-plane   16h   v1.24.0+k0s   10.0.49.188   <none>        Ubuntu 20.04.4 LTS   5.13.0-1022-aws   containerd://1.6.4
ip-10-0-62-250.eu-west-1.compute.internal   Ready    <none>          16h   v1.24.0+k0s   10.0.62.250   <none>        Ubuntu 20.04.4 LTS   5.13.0-1017-aws   docker://20.10.16
```
