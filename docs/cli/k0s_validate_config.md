## k0s validate config

Validate k0s configuration

### Synopsis

Example:
   k0s validate config --config path_to_config.yaml

```shell
k0s validate config [flags]
```

### Options

```shell
      --debugListenOn string   Http listenOn for Debug pprof handler (default ":6060")
  -h, --help                   help for config
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

* [k0s validate](k0s_validate.md)  - Validation related sub-commands
