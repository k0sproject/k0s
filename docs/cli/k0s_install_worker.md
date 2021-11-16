## k0s install worker

Install k0s worker on a brand-new system. Must be run as root (or with sudo)

```shell
k0s install worker [flags]
```

### Examples

```shell
Worker subcommand allows you to pass in all available worker parameters.
All default values of worker command will be passed to the service stub unless overriden.

Windows flags like "--api-server", "--cidr-range" and "--cluster-dns" will be ignored since install command doesn't yet support Windows services
```

### Options

```shell
      --api-server string           HACK: api-server for the windows worker node
      --cidr-range string           HACK: cidr range for the windows worker node (default "10.96.0.0/12")
      --cluster-dns string          HACK: cluster dns for the windows worker node (default "10.96.0.10")
      --cri-socket string           container runtime socket to use, default to internal containerd. Format: [remote|docker]:[path-to-socket]
      --enable-cloud-provider       Whether or not to enable cloud provider support in kubelet
  -h, --help                        help for worker
      --kubelet-extra-args string   extra args for kubelet
      --labels strings              Node labels, list of key=value pairs
  -l, --logging stringToString      Logging Levels for the different components (default [kube-controller-manager=1,kube-scheduler=1,kubelet=1,kube-proxy=1,etcd=info,containerd=info,konnectivity-server=1,kube-apiserver=1])
      --profile string              worker profile to use on the node (default "default")
      --token-file string           Path to the file containing token.
```

### Options inherited from parent commands

```shell
      --data-dir string                Data Directory for k0s (default: /var/lib/k0s). DO NOT CHANGE for an existing setup, things will break!
  -d, --debug                          Debug logging (default: false)
      --debugListenOn string           Http listenOn for Debug pprof handler (default ":6060")
      --log-flush-frequency duration   Maximum number of seconds between log flushes (default 5s)
      --status-socket string           Full file path to the socket file. (default "/var/lib/k0s/run/status.sock")
      --version version[=true]         Print version information and quit
```

### SEE ALSO

* [k0s install](k0s_install.md) - Install k0s on a brand-new system. Must be run as root (or with sudo)
