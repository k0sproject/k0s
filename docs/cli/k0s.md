## k0s

k0s - Zero Friction Kubernetes

### Synopsis

k0s - The zero friction Kubernetes - https://k0sproject.io

### Options

```shell
      --data-dir string                Data Directory for k0s (default: /var/lib/k0s). DO NOT CHANGE for an existing setup, things will break!
      --debug                          Debug logging (default: false)
  -h, --help                           help for k0s
      --log-flush-frequency duration   Maximum number of seconds between log flushes (default 5s)
      --version version[=true]         Print version information and quit
```

### SEE ALSO

* [k0s airgap](k0s_airgap.md) - Manage airgap setup
* [k0s api](k0s_api.md) - Run the controller api
* [k0s backup](k0s_backup.md) - Back-Up k0s configuration. Must be run as root (or with sudo)
* [k0s completion](k0s_completion.md) - Generate completion script
* [k0s controller](k0s_controller.md) - Run controller
* [k0s ctr](k0s_ctr.md) - containerd CLI
* [k0s default-config](k0s_default-config.md) - Output the default k0s configuration yaml to stdout
* [k0s docs](k0s_docs.md) - Generate k0s command documentation
* [k0s etcd](k0s_etcd.md) - Manage etcd cluster
* [k0s install](k0s_install.md) - Install k0s on a brand-new system. Must be run as root (or with sudo)
* [k0s kubeconfig](k0s_kubeconfig.md) - Create a kubeconfig file for a specified user
* [k0s kubectl](k0s_kubectl.md) - kubectl controls the Kubernetes cluster manager
* [k0s reset](k0s_reset.md)  - Uninstall k0s. Must be run as root (or with sudo)
* [k0s restore](k0s_restore.md)  - restore k0s state from given backup archive. Must be run as root (or with sudo)
* [k0s start](k0s_start.md)  - Start the k0s service configured on this host. Must be run as root (or with sudo)
* [k0s status](k0s_status.md) - Get k0s instance status information
* [k0s stop](k0s_stop.md) - Stop the k0s service configured on this host. Must be run as root (or with sudo)
* [k0s sysinfo](k0s_sysinfo.md) - Display system information
* [k0s token](k0s_token.md) - Manage join tokens
* [k0s validate](k0s_validate.md) - Validation related sub-commands
* [k0s version](k0s_version.md) - Print the k0s version
* [k0s worker](k0s_worker.md) - Run worker