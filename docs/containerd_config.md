# containerd configuration

[containerd](https://github.com/containerd/containerd) is an industry-standard container runtime.

**NOTE:** In most use cases changes to the containerd configuration will not be required. 

In order to make changes to containerd configuration first you need to generate a default containerd configuration by running:
```
containerd config default > /etc/k0s/containerd.toml
```
This command will set the default values to `/etc/k0s/containerd.toml`. 

`k0s` runs containerd with the follwoing default values:
```
/var/lib/k0s/bin/containerd \
    --root=/var/lib/k0s/containerd \
    --state=/var/lib/k0s/run/containerd \
    --address=/var/lib/k0s/run/containerd.sock \
    --config=/etc/k0s/containerd.toml
```

Before proceeding further, add the following default values to the configuration file:
```
version = 2
root = "/var/lib/k0s/containerd"
state = "/var/lib/k0s/run/containerd"
...

[grpc]
  address = "/var/lib/k0s/run/containerd.sock"
```

Next if you want to change CRI look into this section

 ``` 
  [plugins."io.containerd.runtime.v1.linux"]
    shim = "containerd-shim"
    runtime = "runc"
```

## Using gVisor

> [gVisor](https://gvisor.dev/docs/) is an application kernel, written in Go, that implements a substantial portion of the Linux system call interface. It provides an additional layer of isolation between running applications and the host operating system.

First you must install the needed gVisor binaries into the host.
```sh
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

See gVisor [install docs](https://gvisor.dev/docs/user_guide/install/)

Next we need to prepare the config for `k0s` managed containerD to utilize gVisor as additional runtime:
```sh
cat <<EOF | sudo tee /etc/k0s/containerd.toml
disabled_plugins = ["restart"]
[plugins.linux]
  shim_debug = true
[plugins.cri.containerd.runtimes.runsc]
  runtime_type = "io.containerd.runsc.v1"
EOF
```

Then we can start and join the worker as normally into the cluster:
```sh
k0s worker $token
```

By default containerd uses nromal runc as the runtime. To make gVisor runtime usable for workloads we must register it to Kubernetes side:
```sh
cat <<EOF | kubectl apply -f -
apiVersion: node.k8s.io/v1beta1
kind: RuntimeClass
metadata:
  name: gvisor
handler: runsc
EOF
```

After this we can use it for our workloads:
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

We can verify the created nginx pod is actually running under gVisor runtime:
```
# kubectl exec nginx-gvisor -- dmesg | grep -i gvisor
[    0.000000] Starting gVisor...
```

## Using custom `nvidia-container-runtime`

By default CRI is set tu runC and if you want to configure Nvidia GPU support you will have to replace `runc` with `nvidia-container-runtime` as shown below:

```
[plugins."io.containerd.runtime.v1.linux"]
    shim = "containerd-shim"
    runtime = "nvidia-container-runtime"
```

**Note** To run `nvidia-container-runtime` on your node please look [here](https://josephb.org/blog/containerd-nvidia/) for detailed instructions.


After changes to the configuration, restart `k0s` and in this case containerd will be using newly configured runtime.