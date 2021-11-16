## k0s install

Install k0s on a brand-new system. Must be run as root (or with sudo)

### Options

```shell
      --debugListenOn string   Http listenOn for Debug pprof handler (default ":6060")
  -h, --help                   help for install
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
* [k0s install controller](k0s_install_controller.md) - Install k0s controller on a brand-new system. Must be run as root (or with sudo)
* [k0s install worker](k0s_install_worker.md) - Install k0s worker on a brand-new system. Must be run as root (or with sudo)
