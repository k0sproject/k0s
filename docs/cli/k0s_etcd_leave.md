## k0s etcd leave

Sign off a given etc node from etcd cluster

```shell
k0s etcd leave [flags]
```

### Options

```shell
  -h, --help                  help for leave
      --peer-address string   etcd peer address
```

### Options inherited from parent commands

```shell
  -c, --config string            config file (default: ./k0s.yaml)
      --data-dir string          Data Directory for k0s (default: /var/lib/k0s). DO NOT CHANGE for an existing setup, things will break!
  -d, --debug                    Debug logging (default: false)
      --debugListenOn string     Http listenOn for debug pprof handler (default ":6060")
  -l, --logging stringToString   Logging Levels for the different components (default [konnectivity-server=1,kube-apiserver=1,kube-controller-manager=1,kube-scheduler=1,kubelet=1,kube-proxy=1,etcd=info,containerd=info])
```

### SEE ALSO

* [k0s etcd](k0s_etcd.md) - Manage etcd cluster
