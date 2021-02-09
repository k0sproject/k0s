# Troubleshooting

There are few common cases we've seen where k0s fails to run properly. 

## CoreDNS in crashloop

The most common case we've encountered so far has been CoreDNS getting into crashloop on the node(s).

With kubectl you see something like this:
```sh
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
```
$ kubectl -n kube-system logs coredns-5c98d7d4d8-tfs4q
plugin/loop: Loop (127.0.0.1:55953 -> :1053) detected for zone ".", see https://coredns.io/plugins/loop#troubleshooting. Query: "HINFO 4547991504243258144.3688648895315093531."
```

This is most often caused by systemd-resolved stub (or something similar) running locally and CoreDNS detects a possible loop with DNS queries.

The easiest but most crude way to workaround is to disable the systemd-resolved stub and revert the hosts `/etc/resolv.conf` to original

Read more at CoreDNS [troubleshooting docs](https://coredns.io/plugins/loop/#troubleshooting-loops-in-kubernetes-clusters).

## `k0s controller` fails on ARM boxes

In the logs you probably see ETCD not starting up properly.

Etcd is [not fully supported](https://github.com/etcd-io/etcd/blob/master/Documentation/op-guide/supported-platform.md#current-support) on ARM architecture, thus you need to run `k0s controller` and thus also etcd process with env `ETCD_UNSUPPORTED_ARCH=arm64`.

As Etcd is not fully supported on ARM architecture it also means that k0s controlplane with etcd itself is not fully supported on ARM either.


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

`k0s worker --profile coreos [TOKEN]`


## Profiling

We drop any debug related information and symbols from the compiled binary by utilzing `-w -s` linker flags.

To keep those symbols use `DEBUG` env variable:

```
$ DEBUG=true make k0s
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags=" -X github.com/k0sproject/k0s/pkg/build.Version=v0.9.0-7-g97e5bac -X \"github.com/k0sproject/k0s/pkg/build.EulaNotice=\" -X github.com/k0sproject/k0s/pkg/telemetry.segmentToken=" \
		    -o k0s.code main.go
``` 

Any value not equal to the "false" would work.

To add custom linker flags use `LDFLAGS` variable.

```
$ LD_FLAGS="--custom-flag=value" make k0s
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="--custom-flag=value -X github.com/k0sproject/k0s/pkg/build.Version=v0.9.0-7-g97e5bac -X \"github.com/k0sproject/k0s/pkg/build.EulaNotice=\" -X github.com/k0sproject/k0s/pkg/telemetry.segmentToken=" \
        -o k0s.code main.go
```