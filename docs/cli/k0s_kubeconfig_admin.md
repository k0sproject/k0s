## k0s kubeconfig admin

Display Admin's Kubeconfig file

### Synopsis

Print kubeconfig for the Admin user to stdout

```
k0s kubeconfig admin [command] [flags]
```

### Examples

```
	$ k0s kubeconfig admin > ~/.kube/config
	$ export KUBECONFIG=~/.kube/config
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
      --debugListenOn string     Http listenOn for debug pprof handler (default ":6060")
  -l, --logging stringToString   Logging Levels for the different components (default [konnectivity-server=1,kube-apiserver=1,kube-controller-manager=1,kube-scheduler=1,kubelet=1,kube-proxy=1,etcd=info,containerd=info])
```

### SEE ALSO

* [k0s kubeconfig](k0s_kubeconfig.md)	 - Create a kubeconfig file for a specified user

