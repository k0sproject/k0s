## k0s install server

Helper command for setting up k0s as server node on a brand-new system. Must be run as root (or with sudo)

```
k0s install server [flags]
```

### Examples

```
All default values of server command will be passed to the service stub unless overriden. 

With server subcommand you can setup a single node cluster by running:

	k0s install server --enable-worker
	
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
  -l, --logging stringToString   Logging Levels for the different components (default [containerd=info,konnectivity-server=1,kube-apiserver=1,kube-controller-manager=1,kube-scheduler=1,kubelet=1,kube-proxy=1,etcd=info])
```

### SEE ALSO

* [k0s install](k0s_install.md)	 - Helper command for setting up k0s on a brand-new system. Must be run as root (or with sudo)

