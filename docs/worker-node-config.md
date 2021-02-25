# Configuration options for worker nodes

Currently `k0s worker` command does not take in any special yaml configuration. There still is ways how to configure the workers, the following chapters provide instructions for ways you can configure how the worker runs various components.

## Node labels

`k0s worker` command accepts `--labels` flag with which you can make the newly joined worker node the register itself in the Kubernetes API with the given set of labels.

So for example when running the worker with `k0s worker --token-file k0s.token --labels="k0sproject.io/foo=bar,k0sproject.io/other=xyz"` will result in:
```
/ # kubectl get node --show-labels
NAME      STATUS     ROLES    AGE   VERSION        LABELS
worker0   NotReady   <none>   10s   v1.20.2-k0s1   beta.kubernetes.io/arch=amd64,beta.kubernetes.io/os=linux,k0sproject.io/foo=bar,k0sproject.io/other=xyz,kubernetes.io/arch=amd64,kubernetes.io/hostname=worker0,kubernetes.io/os=linux
```

**Note:** Setting the labels is only effective on the first registration of the node and changing them afterwards has no effect.


## Kubelet args

`k0s worker` command accepts a generic flag to pass in any set of argument for kubelet process.

For example running `k0s worker --token-file=k0s.token --kubelet-extra-args="--node-ip=1.2.3.4 --address=0.0.0.0"` will "pass on" the given flags to kubelet as-is. As the flags are passed as-is make sure you are passing in properly formatted and valued flags as k0s will NOT validate those at all.

