# Runtime

k0s uses [containerd](https://github.com/containerd/containerd) as the default Container Runtime Interface (CRI) and runc as the default low-level runtime. In most cases they don't require any configuration changes. However, if custom configuration is needed, this page provides some examples.

![k0s_runtime](img/k0s_runtime.png)

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

From 1.27.0 onwards k0s enables dynamic configuration on containerd CRI runtimes. This works by k0s creating a special directory in `/etc/k0s/containerd.d/` where user can drop-in partial containerd configuration snippets.

k0s will automatically pick up these files and adds these in containerd configuration `imports` list. If k0s sees the configuration drop-ins are CRI related configurations k0s will automatically collect all these into a single file and adds that as a single import file. This is to overcome some hard limitation on containerd 1.X versions. Read more at [containerd#8056](https://github.com/containerd/containerd/pull/8056)

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

2. Prepare the config for `k0s` managed containerD, to utilize gVisor as additional runtime:

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

#### Using `nvidia-container-runtime`

First, install the NVIDIA runtime components:

```shell
distribution=$(. /etc/os-release;echo $ID$VERSION_ID) \
   && curl -s -L https://nvidia.github.io/nvidia-docker/gpgkey | sudo apt-key add - \
   && curl -s -L https://nvidia.github.io/nvidia-docker/$distribution/nvidia-docker.list | sudo tee /etc/apt/sources.list.d/nvidia-docker.list
sudo apt-get update && sudo apt-get install -y nvidia-container-runtime
```

Next, drop in the containerd runtime configuration snippet into `/etc/k0s/containerd.d/nvidia.toml`

```toml
[plugins."io.containerd.grpc.v1.cri".containerd.runtimes.nvidia]
    privileged_without_host_devices = false
    runtime_engine = ""
    runtime_root = ""
    runtime_type = "io.containerd.runc.v1"
[plugins."io.containerd.grpc.v1.cri".containerd.runtimes.nvidia.options]
    BinaryName = "/usr/bin/nvidia-container-runtime"
```

Create the needed `RuntimeClass`:

```shell
cat <<EOF | kubectl apply -f -
apiVersion: node.k8s.io/v1
kind: RuntimeClass
metadata:
  name: nvidia
handler: nvidia
EOF
```

**Note** Detailed instruction on how to run `nvidia-container-runtime` on your node is available [here](https://docs.nvidia.com/datacenter/cloud-native/kubernetes/install-k8s.html#install-nvidia-container-toolkit-nvidia-docker2).

## Using custom CRI runtime

**Warning**: You can use your own CRI runtime with k0s (for example, `docker`). However, k0s will not start or manage the runtime, and configuration is solely your responsibility.

Use the option `--cri-socket` to run a k0s worker with a custom CRI runtime. the option takes input in the form of `<type>:<socket_path>` (for `type`, use `docker` for a pure Docker setup and `remote` for anything else).

### Using dockershim

To run k0s with a pre-existing Dockershim setup, run the worker with `k0s worker --cri-socket docker:unix:///var/run/cri-dockerd.sock <token>`.
A detailed explanation on dockershim and a guide for installing cri-dockerd can be found in our [k0s dockershim guide](./dockershim.md).
