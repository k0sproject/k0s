## k0s airgap list-images

List image names and version needed for air-gap install

```shell
k0s airgap list-images [flags]
```

### Examples

```shell
k0s airgap list-images
```

### Options

```shell
  -h, --help   help for list-images
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

* [k0s airgap](k0s_airgap.md) - Manage airgap setup
