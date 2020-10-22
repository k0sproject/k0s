# containerd configuration

[containerd](https://github.com/containerd/containerd) is industry-standard container runtime.

**NOTE:** In most use cases changes to the containerd configuration will not be required. 

In order to make changes to containerd configuration first you need to create default containerd configuration by running:
```
containerd config default > /etc/mke/containerd.toml
```
This command will dump default values to `/etc/mke/containerd.toml`. 

Before proceeding further make sure that following values are added to the configuration file:

```
version = 2
root = "/var/lib/mke/containerd"
state = "/run/mke/containerd"
...

[grpc]
  address = "/run/mke/containerd.sock"
```

Next if you want to change CRI look into this section

 ``` 
  [plugins."io.containerd.runtime.v1.linux"]
    shim = "containerd-shim"
    runtime = "runc"
```

By default CRI is set tu runC and if you want to configure Nvidia GPU support you will have to replace `runc` with `nvidia-container-runtime` as shown below:

```
[plugins."io.containerd.runtime.v1.linux"]
    shim = "containerd-shim"
    runtime = "nvidia-container-runtime"
```

**Note** To run `nvidia-container-runtime` on your node please look [here](https://josephb.org/blog/containerd-nvidia/) for detailed instructions.


After changes to the configuration, restart `mke` and in this case containerd will be using newly configured runtime.