## k0s controller

Run controller

```shell
k0s controller [join-token] [flags]
```

### Examples

```shell
Command to associate master nodes:
CLI argument:
$ k0s controller [join-token]

or CLI flag:
$ k0s controller --token-file [path_to_file]
Note: Token can be passed either as a CLI argument or as a flag
```

### Options

```shell
      --api-server string                              HACK: api-server for the windows worker node
      --cidr-range string                              HACK: cidr range for the windows worker node (default "10.96.0.0/12")
      --cluster-dns string                             HACK: cluster dns for the windows worker node (default "10.96.0.10")
  -c, --config string                                  config file, use '-' to read the config from stdin (default "/etc/k0s/k0s.yaml")
      --cri-socket string                              container runtime socket to use, default to internal containerd. Format: [remote|docker]:[path-to-socket]
      --debugListenOn string                           Http listenOn for Debug pprof handler (default ":6060")
      --disable-components strings                     disable components (valid items: konnectivity-server,kube-scheduler,kube-controller-manager,control-api,csr-approver,default-psp,kube-proxy,coredns,network-provider,helm,metrics-server,kubelet-config,system-rbac)
      --enable-cloud-provider                          Whether or not to enable cloud provider support in kubelet
      --enable-dynamic-config                          enable cluster-wide dynamic config based on custom resource
      --enable-k0s-cloud-provider                      enables the k0s-cloud-provider (default false)
      --enable-worker                                  enable worker (default false)
  -h, --help                                           help for controller
      --k0s-cloud-provider-port int                    the port that k0s-cloud-provider binds on (default 10258)
      --k0s-cloud-provider-update-frequency duration   the frequency of k0s-cloud-provider node updates (default 2m0s)
      --kubelet-extra-args string                      extra args for kubelet
      --labels strings                                 Node labels, list of key=value pairs
  -l, --logging stringToString                         Logging Levels for the different components (default [containerd=info,konnectivity-server=1,kube-apiserver=1,kube-controller-manager=1,kube-scheduler=1,kubelet=1,kube-proxy=1,etcd=info])
      --profile string                                 worker profile to use on the node (default "default")
      --single                                         enable single node (implies --enable-worker, default false)
      --status-socket string                           Full file path to the socket file. (default "/var/lib/k0s/run/status.sock")
      --token-file string                              Path to the file containing join-token.
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
