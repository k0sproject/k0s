- [Download the k0s binary](#download-the-k0s-binary)
  - [Prerequisites](#prerequisites)
  - [K0s Download Script](#k0s-download-script)
  - [Installing k0s as a service on the local system](#installing-k0s-as-a-service-on-the-local-system)
  - [Run k0s as a service](#run-k0s-as-a-service)
    - [Check service status](#check-service-status)
    - [Query cluster status](#query-cluster-status)
    - [Fetch nodes](#fetch-nodes)
  - [Enabling Shell Completion](#enabling-shell-completion)
    - [Bash](#bash)
    - [Zsh](#zsh)
    - [Fish](#fish)
  - [Under the hood](#under-the-hood)
  - [Additional Documentation](#additional-documentation)
# Download the k0s binary

## Prerequisites

* [cURL](https://curl.se/) 

Before proceeding, make sure to review the [System Requirements](system-requirements.md)

## K0s Download Script
```
$ curl -sSLf https://get.k0s.sh | sudo sh
```
The download script accepts the following environment variables:

1. `K0S_VERSION=v0.11.0` - select the version of k0s to be installed
2. `DEBUG=true` - outputs commands and their arguments as they are executed.

## Installing k0s as a service on the local system

The `k0s install` sub-command will install k0s as a system service on hosts running one of the supported init systems: Systemd or OpenRC.

Install can be executed for workers, controllers or single node (controller+worker) instances.

The `install controller` sub-command accepts the same flags and parameters as the `k0s controller` sub-command does.

```
$ k0s install controller --help

Helper command for setting up k0s as controller node on a brand-new system. Must be run as root (or with sudo)

Usage:
  k0s install controller [flags]

Aliases:
  controller, server

Examples:
All default values of controller command will be passed to the service stub unless overriden. 

With controller subcommand you can setup a single node cluster by running:

        k0s install controller --enable-worker


Flags:
  -c, --config string            config file (default: ./k0s.yaml)
      --cri-socket string        contrainer runtime socket to use, default to internal containerd. Format: [remote|docker]:[path-to-socket]
  -d, --debug                    Debug logging (default: false)
      --enable-worker            enable worker (default false)
  -h, --help                     help for controller
  -l, --logging stringToString   Logging Levels for the different components (default [konnectivity-server=1,kube-apiserver=1,kube-controller-manager=1,kube-scheduler=1,kubelet=1,kube-proxy=1,etcd=info,containerd=info])
      --profile string           worker profile to use on the node (default "default")
      --token-file string        Path to the file containing join-token.

Global Flags:
      --data-dir string        Data Directory for k0s (default: /var/lib/k0s). DO NOT CHANGE for an existing setup, things will break!
      --debugListenOn string   Http listenOn for debug pprof handler (default ":6060")
```

For example, the command below will install a single node k0s service on Ubuntu 20.10:

```
$ k0s install controller --enable-worker
INFO[2021-02-24 11:05:42] no config file given, using defaults         
INFO[2021-02-24 11:05:42] creating user: etcd                          
INFO[2021-02-24 11:05:42] creating user: kube-apiserver                
INFO[2021-02-24 11:05:42] creating user: konnectivity-server           
INFO[2021-02-24 11:05:42] creating user: kube-scheduler                
INFO[2021-02-24 11:05:42] Installing k0s service
```

## Run k0s as a service

```
$ systemctl start k0scontroller
```

### Check service status

```
$ systemctl status k0scontroller
     Loaded: loaded (/etc/systemd/system/k0scontroller.service; enabled; vendor preset: enabled)
     Active: active (running) since Fri 2021-02-26 08:37:23 UTC; 1min 25s ago
       Docs: https://docs.k0sproject.io
   Main PID: 1408647 (k0s)
      Tasks: 96
     Memory: 1.2G
     CGroup: /system.slice/k0scontroller.service
     ....
```

### Query cluster status

```
$ k0s status
Version: v0.11.0-beta.2-16-g02cddab
Process ID: 9322
Parent Process ID: 1
Role: controller+worker
Init System: linux-systemd
```

### Fetch nodes

```
$ k0s kubectl get nodes
NAME   STATUS   ROLES    AGE    VERSION
k0s    Ready    <none>   4m6s   v1.20.4-k0s1
```


## Enabling Shell Completion
The k0s completion script for Bash, zsh, fish and powershell can be generated with the command `k0s completion < shell >`. Sourcing the completion script in your shell enables k0s autocompletion.

### Bash

```
echo 'source <(k0s completion bash)' >>~/.bashrc
```

```
# To load completions for each session, execute once:
$ k0s completion bash > /etc/bash_completion.d/k0s
```
### Zsh

If shell completion is not already enabled in your environment you will need
to enable it.  You can execute the following once:
```
$ echo "autoload -U compinit; compinit" >> ~/.zshrc
```
```
# To load completions for each session, execute once:
$ k0s completion zsh > "${fpath[1]}/_k0s"
```
You will need to start a new shell for this setup to take effect.

### Fish

```
$ k0s completion fish | source
```
```
# To load completions for each session, execute once:
$ k0s completion fish > ~/.config/fish/completions/k0s.fish
```

## Under the hood

Workers are always run as root. For controllers, the command will create the following system users:
* `etcd`
* `kube-apiserver`
* `konnectivity-server`
* `kube-scheduler`


## Additional Documentation
see: [k0s install](cli/k0s_install.md)
