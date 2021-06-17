## k0s

k0s - Zero Friction Kubernetes

### Synopsis

k0s - The zero friction Kubernetes - https://k0sproject.io

### Options

```shell
  -c, --config string            config file (default: ./k0s.yaml)
      --data-dir string          Data Directory for k0s (default: /var/lib/k0s). DO NOT CHANGE for an existing setup, things will break!
  -d, --debug                    Debug logging (default: false)
      --debugListenOn string     Http listenOn for debug pprof handler (default ":6060")
  -h, --help                     help for k0s
  -l, --logging stringToString   Logging Levels for the different components (default [konnectivity-server=1,kube-apiserver=1,kube-controller-manager=1,kube-scheduler=1,kubelet=1,kube-proxy=1,etcd=info,containerd=info])
```

### SEE ALSO

* [k0s api](k0s_api.md) - Run the controller api
* [k0s completion](k0s_completion.md) - Generate completion script
* [k0s controller](k0s_controller.md) - Run controller
* [k0s default-config](k0s_default-config.md) - Output the default k0s configuration yaml to stdout
* [k0s docs](k0s_docs.md) - Generate Markdown docs for the k0s binary
* [k0s etcd](k0s_etcd.md) - Manage etcd cluster
* [k0s install](k0s_install.md) - Helper command for setting up k0s on a brand-new system. Must be run as root (or with sudo)
* [k0s start](k0s_stop.md) - Start the k0s service after it has been installed using `k0s install`. Must be run as root (or with sudo)
* [k0s stop](k0s_stop.md) - Stop the k0s service after it has been installed using `k0s install`. Must be run as root (or with sudo)
* [k0s kubeconfig](k0s_kubeconfig.md) - Create a kubeconfig file for a specified user
* [k0s status](k0s_status.md) - Helper command for get general information about k0s
* [k0s token](k0s_token.md) - Manage join tokens
* [k0s validate](k0s_validate.md) - Helper command for validating the config file
* [k0s version](k0s_version.md) - Print the k0s version
* [k0s worker](k0s_worker.md) - Run worker
