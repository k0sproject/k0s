# Common Pitfalls

There are few common cases we've seen where k0s fails to run properly.

## CoreDNS in crashloop

The most common case we've encountered so far has been CoreDNS getting into crashloop on the node(s).

With kubectl you see something like this:

```shell
$ kubectl get pod --all-namespaces
NAMESPACE     NAME                                       READY   STATUS    RESTARTS   AGE
kube-system   calico-kube-controllers-5f6546844f-25px6   1/1     Running   0          167m
kube-system   calico-node-fwjx5                          1/1     Running   0          164m
kube-system   calico-node-t4tx5                          1/1     Running   0          164m
kube-system   calico-node-whwsg                          1/1     Running   0          164m
kube-system   coredns-5c98d7d4d8-tfs4q                   1/1     Error     17         167m
kube-system   konnectivity-agent-9jkfd                   1/1     Running   0          164m
kube-system   konnectivity-agent-bvhdb                   1/1     Running   0          164m
kube-system   konnectivity-agent-r6mzj                   1/1     Running   0          164m
kube-system   kube-proxy-kr2r9                           1/1     Running   0          164m
kube-system   kube-proxy-tbljr                           1/1     Running   0          164m
kube-system   kube-proxy-xbw7p                           1/1     Running   0          164m
kube-system   metrics-server-7d4bcb75dd-pqkrs            1/1     Running   0          167m
```

When you check the logs, it'll show something like this:

```shell
kubectl -n kube-system logs coredns-5c98d7d4d8-tfs4q
```

```shell
plugin/loop: Loop (127.0.0.1:55953 -> :1053) detected for zone ".", see https://coredns.io/plugins/loop#troubleshooting. Query: "HINFO 4547991504243258144.3688648895315093531."
```

This is most often caused by systemd-resolved stub (or something similar) running locally and CoreDNS detects a possible loop with DNS queries.

The easiest but most crude way to workaround is to disable the systemd-resolved stub and revert the hosts `/etc/resolv.conf` to original

Read more at CoreDNS [troubleshooting docs](https://coredns.io/plugins/loop/#troubleshooting-loops-in-kubernetes-clusters).

## `k0s controller` fails on ARM boxes

In the logs you probably see etcd not starting up properly.

Etcd is [not fully supported][etcd-platforms] on ARM architecture, thus you need
to run `k0s controller` and thus also etcd process with env
`ETCD_UNSUPPORTED_ARCH=arm`.

As etcd is not fully supported on ARM, it also means that the k0s control plane
with etcd itself is not fully supported on ARM either.

[etcd-platforms]: https://etcd.io/docs/v3.5/op-guide/supported-platform/#current-support

## `k0s` will not start on ZFS-based systems

On ZFS-based systems k0s will fail to start because containerd runs by default in overlayfs mode to manage image layers. This is not compatible with ZFS and requires a custom config of containerd. The following steps should get k0s working on ZFS-based systems:

- check with `$ ctr -a /run/k0s/containerd.sock plugins ls` that the containerd ZFS snapshotter plugin is in `ok` state (should be the case if ZFS kernel modules and ZFS userspace utils are correctly configured):

```console
TYPE                            ID                       PLATFORMS      STATUS    
...
io.containerd.snapshotter.v1    zfs                      linux/amd64    ok
...
```

- create a containerd config according to the [documentation](/runtime): `$ containerd config default > /etc/k0s/containerd.toml`
- modify the line in `/etc/k0s/containerd.toml`:

```toml
...
    [plugins."io.containerd.grpc.v1.cri".containerd]
      snapshotter = "overlayfs"
...
```

to

```toml
...
    [plugins."io.containerd.grpc.v1.cri".containerd]
      snapshotter = "zfs"
...
```

- create a ZFS dataset to be used as snapshot storage at your desired location, e.g. `$ zfs create -o mountpoint=/var/lib/k0s/containerd/io.containerd.snapshotter.v1.zfs rpool/containerd`
- install k0s as usual, e.g `$ k0s install controller --single -c /etc/k0s/k0s.yaml`
- containerd should be launched with ZFS support and k0s should initialize the cluster correctly

## Pods pending when using cloud providers

Once we enable [cloud provider support](cloud-providers.md) on kubelet on worker nodes, kubelet will automatically add a taint `node.cloudprovider.kubernetes.io/uninitialized` for the node. This tain will prevent normal workloads to be scheduled on the node until the cloud provider controller actually runs second initialization on the node and removes the taint. This means that these nodes are not available for scheduling until the cloud provider controller is actually successfully running on the cluster.

For troubleshooting your specific cloud provider see its documentation.

## k0s not working with read only `/usr`

By default k0s does not run on nodes where `/usr` is read only.

This can be fixed by changing the default path for `volumePluginDir` in your k0s config. You will need to change to values, one for the kubelet itself, and one for Calico.

Here is a snippet of an example config with the default values changed:

```yaml
spec:
  controllerManager:
    extraArgs:
      flex-volume-plugin-dir: "/etc/kubernetes/kubelet-plugins/volume/exec"
  network:
    calico:
      flexVolumeDriverPath: /etc/k0s/kubelet-plugins/volume/exec/nodeagent~uds
  workerProfiles:
    - name: coreos
      values:
        volumePluginDir: /etc/k0s/kubelet-plugins/volume/exec/
```

With this config you can start your controller as usual. Any workers will need to be started with

```shell
k0s worker --profile coreos [TOKEN]
```

## Profiling

We drop any debug related information and symbols from the compiled binary by utilzing `-w -s` linker flags.

To keep those symbols use `DEBUG` env variable:

```shell
DEBUG=true make k0s
```

Any value not equal to the "false" would work.

To add custom linker flags use `LDFLAGS` variable.

```shell
LD_FLAGS="--custom-flag=value" make k0s
```

## I'm using custom CRI and missing some labels in Prometheus metrics

Due to removal of the embedded docker-shim from Kubelet, the Kubelets embedded [cadvisor](https://github.com/google/cadvisor) metrics got slightly broken. If your container runtime is a custom containerd you can add `--kubelet-extra-flags="--containerd=<path/to/containerd.sock>"` into k0s worker startup. That configures the kubelet embedded cadvisor to talk directly with containerd to gather the metrics and thus gets the expected labels in place.

Unfortunately this does not work on when using Docker via cri-dockerd shim. There's currently no easy workarounds for this.

In the future kubelet will be refactored to get the container metrics from CRI interface rather than from the runtime directly. This work is specified and followed up in [KEP-2371](https://github.com/kubernetes/enhancements/blob/master/keps/sig-node/2371-cri-pod-container-stats/README.md) but until that work completes the only option is to run a standalone cAdvisor. There's ongoing [effort](https://github.com/kubernetes/website/issues/30681) to both document the current shortcomings and how to run standalone cAdvisor in Kubernetes community.

## Customized configurations

- All data directories reside under `/var/lib/k0s`, for example:
  - `/var/lib/k0s/kubelet`
  - `/var/lib/k0s/etcd`
