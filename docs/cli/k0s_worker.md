## k0s worker

Run worker

```shell
k0s worker [join-token] [flags]
```

### Examples

```shell
  Command to add worker node to the master node:
  CLI argument:
  $ k0s worker [token]

  or CLI flag:
  $ k0s worker --token-file [path_to_file]
  Note: Token can be passed either as a CLI argument or as a flag
```

### Options

```shell
      --api-server string           HACK: api-server for the windows worker node
      --cidr-range string           HACK: cidr range for the windows worker node (default "10.96.0.0/12")
      --cluster-dns string          HACK: cluster dns for the windows worker node (default "10.96.0.10")
      --cri-socket string           container runtime socket to use, default to internal containerd. Format: [remote|docker]:[path-to-socket]
      --debugListenOn string        Http listenOn for Debug pprof handler (default ":6060")
      --enable-cloud-provider       Whether or not to enable cloud provider support in kubelet
  -h, --help                        help for worker
      --kubelet-extra-args string   extra args for kubelet
      --labels strings              Node labels, list of key=value pairs
  -l, --logging stringToString      Logging Levels for the different components (default [kube-controller-manager=1,kube-scheduler=1,kubelet=1,kube-proxy=1,etcd=info,containerd=info,konnectivity-server=1,kube-apiserver=1])
      --profile string              worker profile to use on the node (default "default")
      --status-socket string        Full file path to the socket file. (default "/var/lib/k0s/run/status.sock")
      --token-file string           Path to the file containing token.
```

### Options inherited from parent commands

```shell
      --data-dir string                Data Directory for k0s (default: /var/lib/k0s). DO NOT CHANGE for an existing setup, things will break!
      --debug                          Debug logging (default: false)
      --log-flush-frequency duration   Maximum number of seconds between log flushes (default 5s)
      --version version[=true]         Print version information and quit
```

### SEE ALSO

* [k0s](k0s.md) - k0s - Zero Friction Kubernetes