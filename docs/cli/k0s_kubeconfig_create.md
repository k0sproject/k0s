## k0s kubeconfig create

Create a kubeconfig for a user

### Synopsis

Create a kubeconfig with a signed certificate and public key for a given user (and optionally user groups)
Note: A certificate once signed cannot be revoked for a particular user

```
k0s kubeconfig create [username] [flags]
```

### Examples

```
	Command to create a kubeconfig for a user:
	CLI argument:
	$ k0s kubeconfig create [username]

	optionally add groups:
	$ k0s kubeconfig create [username] --groups [groups]
```

### Options

```
      --groups string   Specify groups
  -h, --help            help for create
```

### Options inherited from parent commands

```
  -c, --config string            config file (default: ./k0s.yaml)
      --data-dir string          Data Directory for k0s (default: /var/lib/k0s). DO NOT CHANGE for an existing setup, things will break!
  -d, --debug                    Debug logging (default: false)
  -l, --logging stringToString   Logging Levels for the different components (default [kube-scheduler=1,kubelet=1,etcd=info,containerd=info,konnectivity-server=1,kube-apiserver=1,kube-controller-manager=1])
```

### SEE ALSO

* [k0s kubeconfig](k0s_kubeconfig.md)	 - Create a kubeconfig for a user

