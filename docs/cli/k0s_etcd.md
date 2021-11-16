## k0s etcd

Manage etcd cluster

### Options

```shell
      --debugListenOn string   Http listenOn for Debug pprof handler (default ":6060")
  -h, --help                   help for etcd
      --status-socket string   Full file path to the socket file. (default "/var/lib/k0s/run/status.sock")
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
* [k0s etcd leave](k0s_etcd_leave.md) - Sign off a given etc node from etcd cluster
* [k0s etcd member-list](k0s_etcd_member-list.md) - Returns etcd cluster members list
