## k0s server

Run server

```
k0s server [join-token] [flags]
```

### Examples

```
	Command to associate master nodes:
	CLI argument:
	$ k0s server [join-token]

	or CLI flag:
	$ k0s server --token-file [path_to_file]
	Note: Token can be passed either as a CLI argument or as a flag
```

### Options

```
      --enable-worker       enable worker (default false)
  -h, --help                help for server
      --profile string      worker profile to use on the node (default "default")
      --token-file string   Path to the file containing join-token.
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

