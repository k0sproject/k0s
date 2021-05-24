## k0s install

Helper command for setting up k0s on a brand-new system. Must be run as root (or with sudo).

### Options

```shell
  -h, --help   help for install
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

* [k0s](k0s.md) - k0s - Zero Friction Kubernetes
* [k0s install controller](k0s_install_controller.md) - Helper command for setting up k0s as controller node on a brand-new system. Must be run as root (or with sudo)
* [k0s install worker](k0s_install_worker.md) - Helper command for setting up k0s as a worker node on a brand-new system. Must be run as root (or with sudo)
* [k0s start](k0s_stop.md) - Start the k0s service after it has been installed using `k0s install`. Must be run as root (or with sudo)
* [k0s stop](k0s_stop.md) - Stop the k0s service after it has been installed using `k0s install`. Must be run as root (or with sudo)
