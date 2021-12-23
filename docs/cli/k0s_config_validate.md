## k0s config validate

Validate k0s configuration

### Synopsis

Example:
   k0s config validate --config path_to_config.yaml

```
k0s config validate [flags]
```

### Options

```
  -c, --config string          config file, use '-' to read the config from stdin
      --debugListenOn string   Http listenOn for Debug pprof handler (default ":6060")
  -h, --help                   help for validate
      --status-socket string   Full file path to the socket file. (default "/var/lib/k0s/run/status.sock")
```

### Options inherited from parent commands

```
      --data-dir string          Data Directory for k0s (default: /var/lib/k0s). DO NOT CHANGE for an existing setup, things will break!
      --debug                    Debug logging (default: false)
      --version version[=true]   Print version information and quit
```

### SEE ALSO

* [k0s config](k0s_config.md)	 - Configuration related sub-commands

