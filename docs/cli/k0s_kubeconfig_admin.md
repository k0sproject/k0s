## k0s kubeconfig admin

Display Admin's Kubeconfig file

### Synopsis

Print kubeconfig for the Admin user to stdout

```shell
k0s kubeconfig admin [flags]
```

### Examples

```shell
k0s kubeconfig admin > ~/.kube/config
export KUBECONFIG=~/.kube/config
kubectl get nodes
```

### Options

```shell
  -h, --help   help for admin
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
