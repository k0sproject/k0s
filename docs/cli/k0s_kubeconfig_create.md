## k0s kubeconfig create

Create a kubeconfig for a user

### Synopsis

Create a kubeconfig with a signed certificate and public key for a given user (and optionally user groups)
Note: A certificate once signed cannot be revoked for a particular user

```shell
k0s kubeconfig create [username] [flags]
```

### Examples

Command to create a kubeconfig for a user:

```shell
k0s kubeconfig create [username]
```

optionally add groups:

```shell
k0s kubeconfig create [username] --groups [groups]
```

### Options

```shell
      --groups string   Specify groups
  -h, --help            help for create
```

### Options inherited from parent commands

```shell
  -c, --config string            config file (default: ./k0s.yaml)
      --data-dir string          Data Directory for k0s (default: /var/lib/k0s). DO NOT CHANGE for an existing setup, things will break!
  -d, --debug                    Debug logging (default: false)
      --debugListenOn string     Http listenOn for debug pprof handler (default ":6060")
  -l, --logging stringToString   Logging Levels for the different components (default [konnectivity-server=1,kube-apiserver=1,kube-controller-manager=1,kube-scheduler=1,kubelet=1,kube-proxy=1,etcd=info,containerd=info])
```

### SEE ALSO

* [k0s kubeconfig](k0s_kubeconfig.md) - Create a kubeconfig file for a specified user
