## k0s reset

Uninstall k0s. Must be run as root (or with sudo)

```shell
k0s reset [flags]
```

### Options

```shell
  -c, --config string          config file, use '-' to read the config from stdin (default "/etc/k0s/k0s.yaml")
      --cri-socket string      container runtime socket to use, default to internal containerd. Format: [remote|docker]:[path-to-socket]
      --debugListenOn string   Http listenOn for Debug pprof handler (default ":6060")
  -h, --help                   help for reset
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
