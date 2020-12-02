## k0s token create

Create join token

```
k0s token create [flags]
```

### Options

```
      --expiry string   set duration time for token (default "0")
  -h, --help            help for create
      --role string     Either worker or controller (default "worker")
      --wait            wait forever (default false)
```

### Options inherited from parent commands

```
  -c, --config string            config file (default: ./k0s.yaml)
      --data-dir string          Data Directory for k0s (default: /var/lib/k0s). DO NOT CHANGE for an existing setup, things will break!
  -d, --debug                    Debug logging (default: false)
  -l, --logging stringToString   Logging Levels for the different components (default [kube-scheduler=1,kubelet=1,etcd=info,containerd=info,konnectivity-server=1,kube-apiserver=1,kube-controller-manager=1])
```

### SEE ALSO

* [k0s token](k0s_token.md)	 - Manage join tokens

