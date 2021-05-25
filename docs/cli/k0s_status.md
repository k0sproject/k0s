## k0s status

Helper command for get general information about k0s

```shell
k0s status [flags]
```

### Synopsis

The command will return information about system init, PID, k0s role, kubeconfig and similar.

### Options

```shell
  -h, --help         help for status
  -o, --out string   sets type of out put to json or yaml
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
