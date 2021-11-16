## k0s restore

restore k0s state from given backup archive. Must be run as root (or with sudo)

```shell
k0s restore [flags]
```

### Options

```shell
      --config-out string      Specify desired name and full path for the restored k0s.yaml file (default: /etc/k0s/k0s.yaml) (default "/etc/k0s/k0s.yaml")
      --debugListenOn string   Http listenOn for Debug pprof handler (default ":6060")
  -h, --help                   help for restore
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
