## k0s token create

Create join token

```shell
k0s token create [flags]
```

### Examples

```shell
k0s token create --role worker --expiry 100h //sets expiration time to 100 hours
k0s token create --role worker --expiry 10m  //sets expiration time to 10 minutes

```

### Options

```shell
      --expiry string   Expiration time of the token. Format 1.5h, 2h45m or 300ms. (default "0s")
  -h, --help            help for create
      --role string     Either worker or controller (default "worker")
      --wait            wait forever (default false)
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

* [k0s token](k0s_token.md) - Manage join tokens
