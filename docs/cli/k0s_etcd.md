## k0s etcd

Manage etcd cluster

### Options

```
  -h, --help   help for etcd
```

### Options inherited from parent commands

```
  -c, --config string            config file (default: ./k0s.yaml)
      --data-dir string          Data Directory for k0s (default: /var/lib/k0s). DO NOT CHANGE for an existing setup, things will break!
  -d, --debug                    Debug logging (default: false)
      --debugListenOn string     Http listenOn for debug pprof handler (default ":6060")
  -l, --logging stringToString   Logging Levels for the different components (default [konnectivity-server=1,kube-apiserver=1,kube-controller-manager=1,kube-scheduler=1,kubelet=1,kube-proxy=1,etcd=info,containerd=info])
```

### SEE ALSO

* [k0s](k0s.md)	 - k0s - Zero Friction Kubernetes
* [k0s etcd leave](k0s_etcd_leave.md)	 - Sign off a given etc node from etcd cluster
* [k0s etcd member-list](k0s_etcd_member-list.md)	 - Returns etcd cluster members list

