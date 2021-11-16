## k0s kubeconfig create

Create a kubeconfig for a user

### Synopsis

Create a kubeconfig with a signed certificate and public key for a given user (and optionally user groups)
Note: A certificate once signed cannot be revoked for a particular user

```shell
k0s kubeconfig create [username] [flags]
```

### Examples

```shell
Command to create a kubeconfig for a user:
CLI argument:
$ k0s kubeconfig create [username]

optionally add groups:
$ k0s kubeconfig create [username] --groups [groups]
```

### Options

```shell
      --groups string   Specify groups
  -h, --help            help for create
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

* [k0s kubeconfig](k0s_kubeconfig.md) - Create a kubeconfig file for a specified user
