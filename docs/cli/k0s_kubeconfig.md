## k0s kubeconfig

Create a kubeconfig for a user

```
k0s kubeconfig [command] [flags]
```

### Options

```
  -h, --help   help for kubeconfig
```

### Options inherited from parent commands

```
  -c, --config string            config file (default: ./k0s.yaml)
      --data-dir string          Data Directory for k0s (default: /var/lib/k0s). DO NOT CHANGE for an existing setup, things will break!
  -d, --debug                    Debug logging (default: false)
  -l, --logging stringToString   Logging Levels for the different components (default [kube-scheduler=1,kubelet=1,etcd=info,containerd=info,konnectivity-server=1,kube-apiserver=1,kube-controller-manager=1])
```

### SEE ALSO

* [k0s](k0s.md)	 - k0s - Zero Friction Kubernetes
* [k0s kubeconfig admin](k0s_kubeconfig_admin.md)	 - Manage user access
* [k0s kubeconfig create](k0s_kubeconfig_create.md)	 - Create a kubeconfig for a user

