## k0s install worker

Helper command for setting up k0s as a worker node on a brand-new system. Must be run as root (or with sudo)

```shell
k0s install worker [flags]
```

### Synopsis

Worker subcommand allows you to pass in all available worker parameters.

All default values of worker command will be passed to the service stub unless overriden.

Windows flags like "--api-server", "--cidr-range" and "--cluster-dns" will be ignored since install command doesn't yet support Windows services

### Options

```shell
      --api-server string       HACK: api-server for the windows worker node
      --cidr-range string       HACK: cidr range for the windows worker node (default "10.96.0.0/12")
      --cluster-dns string      HACK: cluster dns for the windows worker node (default "10.96.0.10")
      --cri-socket string       contrainer runtime socket to use, default to internal containerd. Format: [remote|docker]:[path-to-socket]
      --enable-cloud-provider   Whether or not to enable cloud provider support in kubelet
  -h, --help                    help for worker
      --profile string          worker profile to use on the node (default "default")
      --token-file string       Path to the file containing token.
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

* [k0s install](k0s_install.md) - Helper command for setting up k0s on a brand-new system. Must be run as root (or with sudo)
