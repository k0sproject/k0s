# Node-local load balancing

**Note:** This feature is experimental! Expect instabilities and/or breaking
changes.

For clusters that don't have an [externally managed load balancer] for the k0s
control plane, there is another option to get a highly available control plane,
at least from within the cluster. K0s calls this "node-local load balancing". In
contrast to an externally managed load balancer, node-local load balancing takes
place exclusively on the worker nodes. It does not contribute to making the
control plane highly available to the outside world (e.g. humans interacting
with the cluster using management tools such as [Lens](https://k8slens.dev/) or
`kubectl`), but rather makes the cluster itself internally resilient to
controller node outages.

[externally managed load balancer]: high-availability.md#load-balancer

## Technical functionality

The k0s worker process manages a load balancer on each worker node's loopback
interface and configures the relevant components to use that load balancer. This
allows for requests from worker components to the control plane to be
distributed among all currently available controller nodes, rather than being
directed to the controller node that has been used to join a particular worker
into the cluster. This improves the reliability and fault tolerance of the
cluster in case a controller node becomes unhealthy.

[Envoy](https://www.envoyproxy.io/) is the only load balancer that is supported
so far. Please note that Envoy is not available on ARMv7, so node-local load
balancing is currently unavailable on that platform.

## Enabling in a cluster

In order to use node-local load balancing, the cluster needs to comply with the
following:

* The cluster doesn't use an externally managed load balancer, i.e. the cluster
  configuration doesn't specify a non-empty
  [`spec.api.externalAddress`][specapi].
* K0s isn't running as a [single node](k0s-single-node.md), i.e. it isn't
  started using the `--single` flag.
* The cluster should have multiple controller nodes. Node-local load balancing
  also works with a single controller node, but is only useful in conjunction
  with a highly available control plane.

Add the following to the cluster configuration (`k0s.yaml`):

```yaml
spec:
  network:
    nodeLocalLoadBalancing:
      enabled: true
      type: EnvoyProxy
```

Or alternatively, if using [`k0sctl`](k0sctl-install.md), add the following to
the k0sctl configuration (`k0sctl.yaml`):

```yaml
spec:
  k0s:
    config:
      spec:
        network:
          nodeLocalLoadBalancing:
            enabled: true
            type: EnvoyProxy
```

All newly added worker nodes will then use node-local load balancing. The k0s
worker process on worker nodes that are already running must be restarted for
the new configuration to take effect.

[specapi]: configuration.md#specapi

## Full example using `k0sctl`

The following example shows a full `k0sctl` configuration file featuring three
controllers and two workers with node-local load balancing enabled:

```yaml
apiVersion: k0sctl.k0sproject.io/v1beta1
kind: Cluster
metadata:
  name: k0s-cluster
spec:
  k0s:
    version: v{{{ extra.k8s_version }}}+k0s.0
    config:
      spec:
        network:
          nodeLocalLoadBalancing:
            enabled: true
            type: EnvoyProxy
  hosts:
    - role: controller
      ssh:
        address: 10.81.146.254
        keyPath: k0s-ssh-private-key.pem
        port: 22
        user: k0s
    - role: controller
      ssh:
        address: 10.81.146.184
        keyPath: k0s-ssh-private-key.pem
        port: 22
        user: k0s
    - role: controller
      ssh:
        address: 10.81.146.113
        keyPath: k0s-ssh-private-key.pem
        port: 22
        user: k0s
    - role: worker
      ssh:
        address: 10.81.146.198
        keyPath: k0s-ssh-private-key.pem
        port: 22
        user: k0s
    - role: worker
      ssh:
        address: 10.81.146.51
        keyPath: k0s-ssh-private-key.pem
        port: 22
        user: k0s
```

Save the above configuration into a file called `k0sctl.yaml` and apply it in
order to bootstrap the cluster:

```console
$ k0sctl apply
⣿⣿⡇⠀⠀⢀⣴⣾⣿⠟⠁⢸⣿⣿⣿⣿⣿⣿⣿⡿⠛⠁⠀⢸⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⠀█████████ █████████ ███
⣿⣿⡇⣠⣶⣿⡿⠋⠀⠀⠀⢸⣿⡇⠀⠀⠀⣠⠀⠀⢀⣠⡆⢸⣿⣿⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀███          ███    ███
⣿⣿⣿⣿⣟⠋⠀⠀⠀⠀⠀⢸⣿⡇⠀⢰⣾⣿⠀⠀⣿⣿⡇⢸⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⠀███          ███    ███
⣿⣿⡏⠻⣿⣷⣤⡀⠀⠀⠀⠸⠛⠁⠀⠸⠋⠁⠀⠀⣿⣿⡇⠈⠉⠉⠉⠉⠉⠉⠉⠉⢹⣿⣿⠀███          ███    ███
⣿⣿⡇⠀⠀⠙⢿⣿⣦⣀⠀⠀⠀⣠⣶⣶⣶⣶⣶⣶⣿⣿⡇⢰⣶⣶⣶⣶⣶⣶⣶⣶⣾⣿⣿⠀█████████    ███    ██████████
k0sctl 0.17.2 Copyright 2023, k0sctl authors.
By continuing to use k0sctl you agree to these terms:
https://k0sproject.io/licenses/eula
level=info msg="==> Running phase: Connect to hosts"
level=info msg="[ssh] 10.81.146.254:22: connected"
level=info msg="[ssh] 10.81.146.184:22: connected"
level=info msg="[ssh] 10.81.146.113:22: connected"
level=info msg="[ssh] 10.81.146.51:22: connected"
level=info msg="[ssh] 10.81.146.198:22: connected"
level=info msg="==> Running phase: Detect host operating systems"
level=info msg="[ssh] 10.81.146.254:22: is running Alpine Linux v3.17"
level=info msg="[ssh] 10.81.146.113:22: is running Alpine Linux v3.17"
level=info msg="[ssh] 10.81.146.184:22: is running Alpine Linux v3.17"
level=info msg="[ssh] 10.81.146.198:22: is running Alpine Linux v3.17"
level=info msg="[ssh] 10.81.146.51:22: is running Alpine Linux v3.17"
level=info msg="==> Running phase: Acquire exclusive host lock"
level=info msg="==> Running phase: Prepare hosts"
level=info msg="[ssh] 10.81.146.113:22: installing packages (curl)"
level=info msg="[ssh] 10.81.146.198:22: installing packages (curl, iptables)"
level=info msg="[ssh] 10.81.146.254:22: installing packages (curl)"
level=info msg="[ssh] 10.81.146.51:22: installing packages (curl, iptables)"
level=info msg="[ssh] 10.81.146.184:22: installing packages (curl)"
level=info msg="==> Running phase: Gather host facts"
level=info msg="[ssh] 10.81.146.184:22: using k0s-controller-1 as hostname"
level=info msg="[ssh] 10.81.146.51:22: using k0s-worker-1 as hostname"
level=info msg="[ssh] 10.81.146.198:22: using k0s-worker-0 as hostname"
level=info msg="[ssh] 10.81.146.113:22: using k0s-controller-2 as hostname"
level=info msg="[ssh] 10.81.146.254:22: using k0s-controller-0 as hostname"
level=info msg="[ssh] 10.81.146.184:22: discovered eth0 as private interface"
level=info msg="[ssh] 10.81.146.51:22: discovered eth0 as private interface"
level=info msg="[ssh] 10.81.146.198:22: discovered eth0 as private interface"
level=info msg="[ssh] 10.81.146.113:22: discovered eth0 as private interface"
level=info msg="[ssh] 10.81.146.254:22: discovered eth0 as private interface"
level=info msg="==> Running phase: Download k0s binaries to local host"
level=info msg="==> Running phase: Validate hosts"
level=info msg="==> Running phase: Gather k0s facts"
level=info msg="==> Running phase: Validate facts"
level=info msg="==> Running phase: Upload k0s binaries to hosts"
level=info msg="[ssh] 10.81.146.254:22: uploading k0s binary from /home/k0sctl/.cache/k0sctl/k0s/linux/amd64/k0s-v{{{ extra.k8s_version }}}+k0s.0"
level=info msg="[ssh] 10.81.146.113:22: uploading k0s binary from /home/k0sctl/.cache/k0sctl/k0s/linux/amd64/k0s-v{{{ extra.k8s_version }}}+k0s.0"
level=info msg="[ssh] 10.81.146.51:22: uploading k0s binary from /home/k0sctl/.cache/k0sctl/k0s/linux/amd64/k0s-v{{{ extra.k8s_version }}}+k0s.0"
level=info msg="[ssh] 10.81.146.198:22: uploading k0s binary from /home/k0sctl/.cache/k0sctl/k0s/linux/amd64/k0s-v{{{ extra.k8s_version }}}+k0s.0"
level=info msg="[ssh] 10.81.146.184:22: uploading k0s binary from /home/k0sctl/.cache/k0sctl/k0s/linux/amd64/k0s-v{{{ extra.k8s_version }}}+k0s.0"
level=info msg="==> Running phase: Configure k0s"
level=info msg="[ssh] 10.81.146.254:22: validating configuration"
level=info msg="[ssh] 10.81.146.184:22: validating configuration"
level=info msg="[ssh] 10.81.146.113:22: validating configuration"
level=info msg="[ssh] 10.81.146.113:22: configuration was changed"
level=info msg="[ssh] 10.81.146.184:22: configuration was changed"
level=info msg="[ssh] 10.81.146.254:22: configuration was changed"
level=info msg="==> Running phase: Initialize the k0s cluster"
level=info msg="[ssh] 10.81.146.254:22: installing k0s controller"
level=info msg="[ssh] 10.81.146.254:22: waiting for the k0s service to start"
level=info msg="[ssh] 10.81.146.254:22: waiting for kubernetes api to respond"
level=info msg="==> Running phase: Install controllers"
level=info msg="[ssh] 10.81.146.254:22: generating token"
level=info msg="[ssh] 10.81.146.184:22: writing join token"
level=info msg="[ssh] 10.81.146.184:22: installing k0s controller"
level=info msg="[ssh] 10.81.146.184:22: starting service"
level=info msg="[ssh] 10.81.146.184:22: waiting for the k0s service to start"
level=info msg="[ssh] 10.81.146.184:22: waiting for kubernetes api to respond"
level=info msg="[ssh] 10.81.146.254:22: generating token"
level=info msg="[ssh] 10.81.146.113:22: writing join token"
level=info msg="[ssh] 10.81.146.113:22: installing k0s controller"
level=info msg="[ssh] 10.81.146.113:22: starting service"
level=info msg="[ssh] 10.81.146.113:22: waiting for the k0s service to start"
level=info msg="[ssh] 10.81.146.113:22: waiting for kubernetes api to respond"
level=info msg="==> Running phase: Install workers"
level=info msg="[ssh] 10.81.146.51:22: validating api connection to https://10.81.146.254:6443"
level=info msg="[ssh] 10.81.146.198:22: validating api connection to https://10.81.146.254:6443"
level=info msg="[ssh] 10.81.146.254:22: generating token"
level=info msg="[ssh] 10.81.146.198:22: writing join token"
level=info msg="[ssh] 10.81.146.51:22: writing join token"
level=info msg="[ssh] 10.81.146.198:22: installing k0s worker"
level=info msg="[ssh] 10.81.146.51:22: installing k0s worker"
level=info msg="[ssh] 10.81.146.198:22: starting service"
level=info msg="[ssh] 10.81.146.51:22: starting service"
level=info msg="[ssh] 10.81.146.198:22: waiting for node to become ready"
level=info msg="[ssh] 10.81.146.51:22: waiting for node to become ready"
level=info msg="==> Running phase: Release exclusive host lock"
level=info msg="==> Running phase: Disconnect from hosts"
level=info msg="==> Finished in 3m30s"
level=info msg="k0s cluster version v{{{ extra.k8s_version }}}+k0s.0 is now installed"
level=info msg="Tip: To access the cluster you can now fetch the admin kubeconfig using:"
level=info msg="     k0sctl kubeconfig"
```

The cluster with the two nodes should be available by now. Setup the kubeconfig
file in order to interact with it:

```shell
k0sctl kubeconfig > k0s-kubeconfig
export KUBECONFIG=$(pwd)/k0s-kubeconfig
```

The three controllers are available and provide API Server endpoints:

```console
$ kubectl -n kube-node-lease get \
    lease/k0s-ctrl-k0s-controller-0 \
    lease/k0s-ctrl-k0s-controller-1 \
    lease/k0s-ctrl-k0s-controller-2 \
    lease/k0s-endpoint-reconciler
NAME                        HOLDER                                                             AGE
k0s-ctrl-k0s-controller-0   9ec2b221890e5ed6f4cc70377bfe809fef5be541a2774dc5de81db7acb2786f1   2m37s
k0s-ctrl-k0s-controller-1   fe45284924abb1bfce674e5a9aa8d647f17c81e53bbab17cf28288f13d5e8f97   2m18s
k0s-ctrl-k0s-controller-2   5ab43278e63fc863b2a7f0fe1aab37316a6db40c5a3d8a17b9d35b5346e23b3d   2m9s
k0s-endpoint-reconciler     9ec2b221890e5ed6f4cc70377bfe809fef5be541a2774dc5de81db7acb2786f1   2m37s

$ kubectl -n default get endpoints
NAME         ENDPOINTS                                                  AGE
kubernetes   10.81.146.113:6443,10.81.146.184:6443,10.81.146.254:6443   2m49s
```

The first controller is the current k0s leader. The two worker nodes can be
listed, too:

```console
$ kubectl get nodes -owide
NAME           STATUS   ROLES    AGE     VERSION       INTERNAL-IP     EXTERNAL-IP   OS-IMAGE             KERNEL-VERSION   CONTAINER-RUNTIME
k0s-worker-0   Ready    <none>   2m16s   v{{{ extra.k8s_version }}}+k0s   10.81.146.198   <none>        Alpine Linux v3.17   5.15.83-0-virt   containerd://1.7.12
k0s-worker-1   Ready    <none>   2m15s   v{{{ extra.k8s_version }}}+k0s   10.81.146.51    <none>        Alpine Linux v3.17   5.15.83-0-virt   containerd://1.7.12
```

There is one node-local load balancer pod running for each worker node:

```console
$ kubectl -n kube-system get pod -owide -l app.kubernetes.io/managed-by=k0s,app.kubernetes.io/component=nllb
NAME                READY   STATUS    RESTARTS   AGE   IP              NODE           NOMINATED NODE   READINESS GATES
nllb-k0s-worker-0   1/1     Running   0          81s   10.81.146.198   k0s-worker-0   <none>           <none>
nllb-k0s-worker-1   1/1     Running   0          85s   10.81.146.51    k0s-worker-1   <none>           <none>
```

The cluster is using node-local load balancing and is able to tolerate the
outage of one controller node. Shutdown the first controller to simulate a
failure condition:

```console
$ ssh -i k0s-ssh-private-key.pem k0s@10.81.146.254 'echo "Powering off $(hostname) ..." && sudo poweroff'
Powering off k0s-controller-0 ...
```

Node-local load balancing provides high availability from within the cluster,
not from the outside. The generated kubeconfig file lists the first controller's
IP as the Kubernetes API server address by default. As this controller is gone
by now, a subsequent call to `kubectl` will fail:

```console
$ kubectl get nodes
Unable to connect to the server: dial tcp 10.81.146.254:6443: connect: no route to host
```

Changing the server address in `k0s-kubeconfig` from the first controller to
another one makes the cluster accessible again. Pick one of the other controller
IP addresses and put that into the kubeconfig file. The addresses are listed
both in `k0sctl.yaml` as well as in the output of `kubectl -n default get
endpoints` above.

```console
$ ssh -i k0s-ssh-private-key.pem k0s@10.81.146.184 hostname
k0s-controller-1

$ sed -i s#https://10\\.81\\.146\\.254:6443#https://10.81.146.184:6443#g k0s-kubeconfig

$ kubectl get nodes -owide
NAME           STATUS   ROLES    AGE     VERSION       INTERNAL-IP     EXTERNAL-IP   OS-IMAGE             KERNEL-VERSION   CONTAINER-RUNTIME
k0s-worker-0   Ready    <none>   3m35s   v{{{ extra.k8s_version }}}+k0s   10.81.146.198   <none>        Alpine Linux v3.17   5.15.83-0-virt   containerd://1.7.12
k0s-worker-1   Ready    <none>   3m34s   v{{{ extra.k8s_version }}}+k0s   10.81.146.51    <none>        Alpine Linux v3.17   5.15.83-0-virt   containerd://1.7.12

$ kubectl -n kube-system get pods -owide -l app.kubernetes.io/managed-by=k0s,app.kubernetes.io/component=nllb
NAME                READY   STATUS    RESTARTS   AGE     IP              NODE           NOMINATED NODE   READINESS GATES
nllb-k0s-worker-0   1/1     Running   0          2m31s   10.81.146.198   k0s-worker-0   <none>           <none>
nllb-k0s-worker-1   1/1     Running   0          2m35s   10.81.146.51    k0s-worker-1   <none>           <none>
```

The first controller is no longer active. Its IP address is not listed in the
`default/kubernetes` Endpoints resource and its k0s controller lease is orphaned:

```console
$ kubectl -n default get endpoints
NAME         ENDPOINTS                               AGE
kubernetes   10.81.146.113:6443,10.81.146.184:6443   3m56s

$ kubectl -n kube-node-lease get \
    lease/k0s-ctrl-k0s-controller-0 \
    lease/k0s-ctrl-k0s-controller-1 \
    lease/k0s-ctrl-k0s-controller-2 \
    lease/k0s-endpoint-reconciler
NAME                        HOLDER                                                             AGE
k0s-ctrl-k0s-controller-0                                                                      4m47s
k0s-ctrl-k0s-controller-1   fe45284924abb1bfce674e5a9aa8d647f17c81e53bbab17cf28288f13d5e8f97   4m28s
k0s-ctrl-k0s-controller-2   5ab43278e63fc863b2a7f0fe1aab37316a6db40c5a3d8a17b9d35b5346e23b3d   4m19s
k0s-endpoint-reconciler     5ab43278e63fc863b2a7f0fe1aab37316a6db40c5a3d8a17b9d35b5346e23b3d   4m47s
```

Despite that controller being unavailable, the cluster remains operational. The
third controller has become the new k0s leader. Workloads will run just fine:

```console
$ kubectl -n default run nginx --image=nginx
pod/nginx created

$ kubectl -n default get pods -owide
NAME    READY   STATUS    RESTARTS   AGE   IP           NODE           NOMINATED NODE   READINESS GATES
nginx   1/1     Running   0          16s   10.244.0.5   k0s-worker-1   <none>           <none>

$ kubectl -n default logs nginx
/docker-entrypoint.sh: /docker-entrypoint.d/ is not empty, will attempt to perform configuration
/docker-entrypoint.sh: Looking for shell scripts in /docker-entrypoint.d/
/docker-entrypoint.sh: Launching /docker-entrypoint.d/10-listen-on-ipv6-by-default.sh
10-listen-on-ipv6-by-default.sh: info: Getting the checksum of /etc/nginx/conf.d/default.conf
10-listen-on-ipv6-by-default.sh: info: Enabled listen on IPv6 in /etc/nginx/conf.d/default.conf
/docker-entrypoint.sh: Launching /docker-entrypoint.d/20-envsubst-on-templates.sh
/docker-entrypoint.sh: Launching /docker-entrypoint.d/30-tune-worker-processes.sh
/docker-entrypoint.sh: Configuration complete; ready for start up
[notice] 1#1: using the "epoll" event method
[notice] 1#1: nginx/1.23.3
[notice] 1#1: built by gcc 10.2.1 20210110 (Debian 10.2.1-6)
[notice] 1#1: OS: Linux 5.15.83-0-virt
[notice] 1#1: getrlimit(RLIMIT_NOFILE): 1048576:1048576
[notice] 1#1: start worker processes
[notice] 1#1: start worker process 28
```
