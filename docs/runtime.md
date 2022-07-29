# Runtime

k0s uses [containerd](https://github.com/containerd/containerd) as the default Container Runtime Interface (CRI) and runc as the default low-level runtime. In most cases they don't require any configuration changes. However, if custom configuration is needed, this page provides some examples.

![k0s_runtime](img/k0s_runtime.png)

## containerd configuration

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

Finally, if you want to change CRI look into:

```toml
  [plugins."io.containerd.runtime.v1.linux"]
    shim = "containerd-shim"
    runtime = "runc"
```

## Using gVisor

[gVisor](https://gvisor.dev/docs/) is an application kernel, written in Go, that implements a substantial portion of the Linux system call interface. It provides an additional layer of isolation between running applications and the host operating system.

1. Install the needed gVisor binaries into the host.

    ```shell
    (
      set -e
      URL=https://storage.googleapis.com/gvisor/releases/release/latest
      wget ${URL}/runsc ${URL}/runsc.sha512 \
        ${URL}/gvisor-containerd-shim ${URL}/gvisor-containerd-shim.sha512 \
        ${URL}/containerd-shim-runsc-v1 ${URL}/containerd-shim-runsc-v1.sha512
      sha512sum -c runsc.sha512 \
        -c gvisor-containerd-shim.sha512 \
        -c containerd-shim-runsc-v1.sha512
      rm -f *.sha512
      chmod a+rx runsc gvisor-containerd-shim containerd-shim-runsc-v1
      sudo mv runsc gvisor-containerd-shim containerd-shim-runsc-v1 /usr/local/bin
    )
    ```

    Refer to the [gVisor install docs](https://gvisor.dev/docs/user_guide/install/) for more information.

2. Prepare the config for `k0s` managed containerD, to utilize gVisor as additional runtime:

    ```shell
    cat <<EOF | sudo tee /etc/k0s/containerd.toml
    disabled_plugins = ["restart"]
    [plugins.linux]
      shim_debug = true
    [plugins.cri.containerd.runtimes.runsc]
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
    apiVersion: node.k8s.io/v1beta1
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

5. (Optional) Verify tht the created nginx pod is running under gVisor runtime:

    ```shell
    # kubectl exec nginx-gvisor -- dmesg | grep -i gvisor
    [    0.000000] Starting gVisor...
    ```

## Using `nvidia-container-runtime`

By default, CRI is set to runC. As such, you must configure Nvidia GPU support by replacing `runc` with `nvidia-container-runtime`:

```toml
[plugins."io.containerd.grpc.v1.cri".containerd]
    default_runtime_name = "nvidia"
[plugins."io.containerd.grpc.v1.cri".containerd.runtimes.nvidia]
    privileged_without_host_devices = false
    runtime_engine = ""
    runtime_root = ""
    runtime_type = "io.containerd.runc.v1"
[plugins."io.containerd.grpc.v1.cri".containerd.runtimes.nvidia.options]
    BinaryName = "/usr/bin/nvidia-container-runtime"
```

**Note** Detailed instruction on how to run `nvidia-container-runtime` on your node is available [here](https://docs.nvidia.com/datacenter/cloud-native/kubernetes/install-k8s.html#install-nvidia-container-toolkit-nvidia-docker2).

After editing the configuration, restart `k0s` to get containerd using the newly configured runtime.

## Using custom CRI runtime

**Warning**: You can use your own CRI runtime with k0s (for example, `docker`). However, k0s will not start or manage the runtime, and configuration is solely your responsibility.

Use the option `--cri-socket` to run a k0s worker with a custom CRI runtime. the option takes input in the form of `<type>:<socket_path>` (for `type`, use `docker` for a pure Docker setup and `remote` for anything else).

### Using dockershim

To run k0s with a pre-existing Dockershim setup, run the worker with `k0s worker --cri-socket docker:unix:///var/run/cri-dockerd.sock <token>`.
A detailed explanation on dockershim and a guide for installing cri-dockerd can be found in our [k0s dockershim guide](./docker-shim.md).
