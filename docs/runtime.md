# Runtime

k0s supports any container runtime that implements the [CRI] specification.

k0s comes bundled with [containerd] as the default Container Runtime Interface (CRI) and [runc] as the default low-level runtime. In most cases they don't require any configuration changes. However, if custom configuration is needed, this page provides some examples.

![k0s_runtime](img/k0s_runtime.png)

[CRI]: https://github.com/kubernetes/cri-api
[containerd]: https://github.com/containerd/containerd
[runc]: https://github.com/opencontainers/runc

## containerd configuration

By default k0s manages the full containerd configuration. User has the option of fully overriding, and thus also managing, the configuration themselves.

### User managed containerd configuration

In the default k0s generated configuration there's a "magic" comment telling k0s it is k0s managed:

```toml
# k0s_managed=true
```

If you wish to take over the configuration management remove this line.

To make changes to containerd configuration you must first generate a default containerd configuration, with the default values set to `/etc/k0s/containerd.toml`:

```shell
containerd config default > /etc/k0s/containerd.toml
```

`k0s` runs containerd with the following default values:

```shell
/var/lib/k0s/bin/containerd \
    --root=/var/lib/k0s/containerd \
    --state=/run/k0s/containerd \
    --address=/run/k0s/containerd.sock \
    --config=/etc/k0s/containerd.toml
```

Next, add the following default values to the configuration file:

```toml
version = 2
root = "/var/lib/k0s/containerd"
state = "/run/k0s/containerd"
...

[grpc]
  address = "/run/k0s/containerd.sock"
```

## k0s managed dynamic runtime configuration

As of 1.27.1, k0s allows dynamic configuration of containerd CRI runtimes. This
works by k0s creating a special directory in `/etc/k0s/containerd.d/` where
users can place partial containerd configuration files.

K0s will automatically pick up these files and add them as containerd
configuration `imports`. If a partial configuration file contains a CRI plugin
configuration section, k0s will instead treat such a file as a [merge patch] to
k0s's default containerd configuration. This is to mitigate [containerd's
decision] to replace rather than merge individual plugin configuration sections
from imported configuration files. However, this behavior [may][containerd#7347]
[change][containerd#9982] in future releases of containerd.

Please note, that in order for drop-ins in `/etc/k0s/containerd.d` to take effect on running configuration, `/etc/k0s/containerd.toml` needs to be k0s managed.

If you change the first magic line (`# k0s_managed=true`) in the `/etc/k0s/containerd.toml` (by accident or on purpose), it automatically becomes "not k0s managed". To make it "k0s managed" again, remove `/etc/k0s/containerd.toml` and restart k0s service on the node, it'll be recreated by k0s.

To confirm that drop-ins are applied to running configuration, check the content of `/run/k0s/containerd-cri.toml`, drop-in specific configuration should be present in this file.

[merge patch]: https://datatracker.ietf.org/doc/html/rfc7396
[containerd's decision]: https://github.com/containerd/containerd/pull/3574/commits/24b9e2c1a0a72a7ad302cdce7da3abbc4e6295cb
[containerd#7347]: https://github.com/containerd/containerd/pull/7347
[containerd#9982]: https://github.com/containerd/containerd/pull/9982

### Examples

Following chapters provide some examples how to configure different runtimes for containerd using k0s managed drop-in configurations.

#### Using gVisor

[gVisor](https://gvisor.dev/docs/) is an application kernel, written in Go, that implements a substantial portion of the Linux system call interface. It provides an additional layer of isolation between running applications and the host operating system.

1. Install the needed gVisor binaries into the host.

    ```shell
    (
      set -e
      ARCH=$(uname -m)
      URL=https://storage.googleapis.com/gvisor/releases/release/latest/${ARCH}
      wget ${URL}/runsc ${URL}/runsc.sha512 \
        ${URL}/containerd-shim-runsc-v1 ${URL}/containerd-shim-runsc-v1.sha512
      sha512sum -c runsc.sha512 \
        -c containerd-shim-runsc-v1.sha512
      rm -f *.sha512
      chmod a+rx runsc containerd-shim-runsc-v1
      sudo mv runsc containerd-shim-runsc-v1 /usr/local/bin
    )
    ```

    Refer to the [gVisor install docs](https://gvisor.dev/docs/user_guide/install/) for more information.

2. Prepare the config for `k0s` managed containerd, to utilize gVisor as additional runtime:

    ```shell
    cat <<EOF | sudo tee /etc/k0s/containerd.d/gvisor.toml
    version = 2

    [plugins."io.containerd.grpc.v1.cri".containerd.runtimes.runsc]
      runtime_type = "io.containerd.runsc.v1"
    EOF
    ```

3. Start and join the worker into the cluster, as normal:

    ```shell
    k0s worker $token
    ```

4. Register containerd to the Kubernetes side to make gVisor runtime usable for workloads (by default, containerd uses normal runc as the runtime):

    ```shell
    cat <<EOF | kubectl apply -f -
    apiVersion: node.k8s.io/v1
    kind: RuntimeClass
    metadata:
      name: gvisor
    handler: runsc
    EOF
    ```

    At this point, you can use gVisor runtime for your workloads:

    ```yaml
    apiVersion: v1
    kind: Pod
    metadata:
      name: nginx-gvisor
    spec:
      runtimeClassName: gvisor
      containers:
      - name: nginx
        image: nginx
    ```

5. (Optional) Verify that the created nginx pod is running under gVisor runtime:

    ```shell
    # kubectl exec nginx-gvisor -- dmesg | grep -i gvisor
    [    0.000000] Starting gVisor...
    ```

#### Using nvidia-container-runtime

First, deploy the NVIDIA GPU operator Helm chart with the following commands on top of your k0s cluster:

```shell
helm repo add nvidia https://helm.ngc.nvidia.com/nvidia
helm repo update
helm install nvidia-gpu-operator -n nvidia-gpu-operator \
  --create-namespace \
  --set operator.defaultRuntime=containerd \
  --set toolkit.env[0].name=CONTAINERD_CONFIG \
  --set toolkit.env[0].value=/etc/k0s/containerd.d/nvidia.toml \
  --set toolkit.env[1].name=CONTAINERD_SOCKET \
  --set toolkit.env[1].value=/run/k0s/containerd.sock \
  --set toolkit.env[2].name=CONTAINERD_RUNTIME_CLASS \
  --set toolkit.env[2].value=nvidia \
  nvidia/gpu-operator
```

With this Helm chart values, NVIDIA GPU operator will deploy both driver and toolkit to the GPU nodes and additionally will configure containerd with NVIDIA specific runtime.

**Note** Detailed instruction on how to deploy NVIDIA GPU operator on your k0s cluster is available [here](https://docs.nvidia.com/datacenter/cloud-native/gpu-operator/latest/getting-started.html).

## Using custom CRI runtimes

**Warning**: You can use your own CRI runtime with k0s (for example, `docker`). However, k0s will not start or manage the runtime, and configuration is solely your responsibility.

Use the option `--cri-socket` to run a k0s worker with a custom CRI runtime. the option takes input in the form of `<type>:<url>` (the only supported type is `remote`).

### Using Docker as the container runtime

As of Kubernetes 1.24, the use of Docker as a container runtime is [no longer
supported][dockershim deprecation] out of the box. However, Mirantis provides
[cri-dockerd], a shim that allows Docker to be controlled via CRI. It's [based
on the dockershim][future of dockershim] that was previously part of upstream
Kubernetes.

#### Configuration

In order to use Docker as the container runtime for k0s, the following steps
need to be taken:

1. Manually install required components.  
  On each `k0s worker` and `k0s controller --enable-worker` node, both
  Docker Engine and cri-dockerd need to be installed manually. Follow the
  official [Docker Engine installation guide][install docker] and [cri-dockerd
  installation instructions][install cri-dockerd].

2. Configure and restart affected k0s nodes.  
  Once installations are complete, the nodes needs to be restarted with the
  `--cri-socket` flag pointing to cri-dockerd's socket, which is typically
  located at `/var/run/cri-dockerd.sock`. For instance, the commands to start a
  node would be as follows:
  
      k0s worker --cri-socket=remote:unix:///var/run/cri-dockerd.sock
  
  or, respectively

  ```console
  k0s controller --enable-worker --cri-socket=remote:unix:///var/run/cri-dockerd.sock
  ```

  When running k0s [as a service](cli/k0s_install.md), consider reinstalling the
  service with the appropriate flags:

  ```console
  sudo k0s install --force worker --cri-socket=remote:unix:///var/run/cri-dockerd.sock
  ```

  or, respectively

  ```console
  sudo k0s install --force controller --enable-worker --cri-socket=remote:unix:///var/run/cri-dockerd.sock
  ```

In scenarios where Docker is managed via systemd, it is crucial that the
`cgroupDriver: systemd` setting is included in the Kubelet configuration. It can
be added to the `workerProfiles` section of the k0s configuration. An example of
how the k0s configuration might look:

```yaml
apiVersion: k0s.k0sproject.io/v1beta1
kind: ClusterConfig
metadata:
  name: k0s
spec:
  workerProfiles:
    - name: systemd-docker-cri
      values:
        cgroupDriver: systemd
```

Note that this is a cluster-wide configuration setting that must be added to
the k0s controller's configuration rather than directly to the workers, or to
the cluster configuration if using [dynamic configuration]. See the [worker
profiles] section of the documentation for more details. When starting workers,
both the `--profile=systemd-docker-cri` and `--cri-socket` flags are required.
The profile name, such as `systemd-docker-cri`, is flexible. Alternatively,
this setting can be applied to the `default` profile, which will apply to all
nodes started without a specific profile. In this case, the `--profile` flag is
not needed.

Please note that there are currently some [pitfalls around container
metrics][cadvisor-metrics] when using cri-dockerd.

[dockershim deprecation]: https://kubernetes.io/blog/2020/12/02/dockershim-faq/
[cri-dockerd]: https://github.com/Mirantis/cri-dockerd
[future of dockershim]: https://www.mirantis.com/blog/the-future-of-dockershim-is-cri-dockerd/
[install docker]: https://docs.docker.com/engine/install/
[install cri-dockerd]: https://github.com/Mirantis/cri-dockerd#using-cri-dockerd
[worker profiles]: worker-node-config.md#worker-profiles
[dynamic configuration]: dynamic-configuration.md
[cadvisor-metrics]: ./troubleshooting.md#using-a-custom-container-runtime-and-missing-labels-in-prometheus-metrics

#### Verification

The successful configuration can be verified by executing the following command:

```console
$ kubectl get nodes -o wide
NAME              STATUS   ROLES    AGE   VERSION       INTERNAL-IP     EXTERNAL-IP   OS-IMAGE             KERNEL-VERSION      CONTAINER-RUNTIME
docker-worker-0   Ready    <none>   15m   v{{{ extra.k8s_version }}}+k0s   172.27.77.155   <none>        Ubuntu 22.04.3 LTS   5.15.0-82-generic   docker://24.0.7
```

On the worker nodes, the Kubernetes containers should be listed as regular
Docker containers:

```console
$ docker ps --format "table {{.ID}}\t{{.Names}}\t{{.State}}\t{{.Status}}"
CONTAINER ID   NAMES                                                                                                   STATE     STATUS
9167a937af28   k8s_konnectivity-agent_konnectivity-agent-9rnj7_kube-system_430027b4-75c3-487c-b94d-efeb7204616d_1      running   Up 14 minutes
b6978162a05d   k8s_metrics-server_metrics-server-7556957bb7-wfg8k_kube-system_5f642105-78c8-450a-bfd2-2021b680b932_1   running   Up 14 minutes
d576abe86c92   k8s_coredns_coredns-85df575cdb-vmdq5_kube-system_6f26626e-d241-4f15-889a-bcae20d04e2c_1                 running   Up 14 minutes
8f268b180c59   k8s_kube-proxy_kube-proxy-2x6jz_kube-system_34a7a8ba-e15d-4968-8a02-f5c0cb3c8361_1                      running   Up 14 minutes
ed0a665ec28e   k8s_POD_konnectivity-agent-9rnj7_kube-system_430027b4-75c3-487c-b94d-efeb7204616d_0                     running   Up 14 minutes
a9861a7beab5   k8s_POD_metrics-server-7556957bb7-wfg8k_kube-system_5f642105-78c8-450a-bfd2-2021b680b932_0              running   Up 14 minutes
898befa4840e   k8s_POD_kube-router-fftkt_kube-system_940ad783-055e-4fce-8ce1-093ca01625b9_0                            running   Up 14 minutes
e80dabc23ce7   k8s_POD_kube-proxy-2x6jz_kube-system_34a7a8ba-e15d-4968-8a02-f5c0cb3c8361_0                             running   Up 14 minutes
430a784b1bdd   k8s_POD_coredns-85df575cdb-vmdq5_kube-system_6f26626e-d241-4f15-889a-bcae20d04e2c_0                     running   Up 14 minutes
```
