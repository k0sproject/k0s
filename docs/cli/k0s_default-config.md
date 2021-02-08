## k0s default-config

Output the default k0s configuration yaml to stdout

```
k0s default-config [flags]
```

### Options

```
  -h, --help   help for default-config
```

### Options inherited from parent commands

```
  -c, --config string            config file (default: ./k0s.yaml)
      --data-dir string          Data Directory for k0s (default: /var/lib/k0s). DO NOT CHANGE for an existing setup, things will break!
  -d, --debug                    Debug logging (default: false)
      --debugListenOn string     Http listenOn for debug pprof handler (default ":6060")
  -l, --logging stringToString   Logging Levels for the different components (default [containerd=info,konnectivity-server=1,kube-apiserver=1,kube-controller-manager=1,kube-scheduler=1,kubelet=1,kube-proxy=1,etcd=info])
```

### SEE ALSO

* [k0s](k0s.md)	 - k0s - Zero Friction Kubernetes

