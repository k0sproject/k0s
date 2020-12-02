## k0s kubeconfig admin

Manage user access

### Synopsis

Command dumps admin kubeconfig.

```
k0s kubeconfig admin [command] [flags]
```

### Examples

```
	$ k0s kubeconfig admin > kubeconfig
	$ export KUBECONFIG=kubeconfig
	$ kubectl get nodes
```

### Options

```
  -h, --help   help for admin
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

