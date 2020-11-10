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

## `k0s server` fails on ARM boxes

In the logs you probably see ETCD not starting up properly.

Etcd is [not fully supported](https://github.com/etcd-io/etcd/blob/master/Documentation/op-guide/supported-platform.md#current-support) on ARM architecture, thus you need to run `k0s server` and thus also etcd process with env `ETCD_UNSUPPORTED_ARCH=arm64`.

As Etcd is not fully supported on ARM architecture it also means that k0s controlplane with etcd itself is not fully supported on ARM either.


## Pods pending when using cloud providers

Once we enable [cloud provider support](cloud-providers.md) on kubelet on worker nodes, kubelet will automatically add a taint `node.cloudprovider.kubernetes.io/uninitialized` for the node. This tain will prevent normal workloads to be scheduled on the node untill the cloud provider controller actually runs second initialization on the node and removes the taint. This means that these nodes are not schedulable untill the cloud provider controller is actually succesfully running on the cluster.

For troubleshooting your specific cloud provider see its documentation.