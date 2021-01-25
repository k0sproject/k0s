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
      --cri-socket string   contrainer runtime socket to use, default to internal containerd. Format: [remote|docker]:[path-to-socket]
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
      --debugListenOn string     Http listenOn for debug pprof handler (default ":6060")
  -l, --logging stringToString   Logging Levels for the different components (default [kube-controller-manager=1,kube-scheduler=1,kubelet=1,kube-proxy=1,etcd=info,containerd=info,konnectivity-server=1,kube-apiserver=1])
```

### SEE ALSO

* [k0s](k0s.md)	 - k0s - Zero Friction Kubernetes

