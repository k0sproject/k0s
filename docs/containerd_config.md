# containerd configuration

[containerd](https://github.com/containerd/containerd) is industry-standard container runtime.

In order to make changes to containerd configuration first you need to create default containerd configuration by running:
```
containerd config default > /etc/mke/containerd.toml
```
This command will dump default values to /etc/mke/containerd.toml. 

Next if you want to change CRI look into this section

 ``` 
  [plugins."io.containerd.runtime.v1.linux"]
    shim = "containerd-shim"
    runtime = "runc"
```

By default CRI is set tu runC and for example if you want to configure Nvidia GPU support you will have to replace `runc` with `nvidia-container-runtime`. 

```
[plugins."io.containerd.runtime.v1.linux"]
    shim = "containerd-shim"
    runtime = "nvidia-container-runtime"
```

**Note** To run `nvidia-container-runtime` on your node please look [here](https://josephb.org/blog/containerd-nvidia/) for details how to do it.

Now restart `mke` and containerd will be using newly configure runtime.