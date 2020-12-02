## k0s install

Helper command for setting up k0s on a brand-new system. Must be run as root (or with sudo)

```
k0s install [flags]
```

### Options

```
  -h, --help          help for install
      --role string   node role (possible values: server or worker. In a single-node setup, a worker role should be used) (default "server")
```

### Options inherited from parent commands

```
  -c, --config string            config file (default: ./k0s.yaml)
      --data-dir string          Data Directory for k0s (default: /var/lib/k0s). DO NOT CHANGE for an existing setup, things will break!
  -d, --debug                    Debug logging (default: false)
  -l, --logging stringToString   Logging Levels for the different components (default [kube-scheduler=1,kubelet=1,etcd=info,containerd=info,konnectivity-server=1,kube-apiserver=1,kube-controller-manager=1])
```

### SEE ALSO

* [k0s](k0s.md)	 - k0s - Zero Friction Kubernetes

