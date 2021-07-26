## k0s controller

Run controller

```shell
k0s controller [join-token] [flags]
```

### Examples

Command to associate master nodes using a CLI argument:

```shell
k0s controller [join-token]
```

or a CLI flag:

```shell
k0s controller --token-file [path_to_file]
```

### Options

```shell
      --cri-socket string                              contrainer runtime socket to use, default to internal containerd. Format: [remote|docker]:[path-to-socket]
      --enable-worker                                  enable worker (default false)
  -h, --help                                           help for controller
      --profile string                                 worker profile to use on the node (default "default")
      --token-file string                              Path to the file containing join-token.
      --enable-k0s-cloud-provider                      enables the k0s-cloud-provider (default false)
      --k0s-cloud-provider-port int                    the port that k0s-cloud-provider binds on (default 10258)
      --k0s-cloud-provider-update-frequency duration   the frequency of k0s-cloud-provider node updates (default 2m0s)
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
