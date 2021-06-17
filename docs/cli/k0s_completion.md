## k0s completion

Generate completion script

### Synopsis

To load completions:

#### Bash

```shell
source <(k0s completion bash)
```

To load completions for each session, execute once:

```shell
k0s completion bash > /etc/bash_completion.d/k0s
```

#### Zsh

If shell completion is not already enabled in your environment you will need to enable it. You can execute the following once:

```shell
echo "autoload -U compinit; compinit" >> ~/.zshrc
```

To load completions for each session, execute once:

```shell
k0s completion zsh > "${fpath[1]}/_k0s"
```

You will need to start a new shell for this setup to take effect.

#### Fish

```shell
k0s completion fish | source
```

To load completions for each session, execute once:

```shell
k0s completion fish > ~/.config/fish/completions/k0s.fish
```

### Usage

```shell
k0s completion [bash|zsh|fish|powershell]
```

### Options

```shell
  -h, --help   help for completion
```

### Options inherited from parent commands

```shell
  -c, --config string            config file (default: ./k0s.yaml)
      --data-dir string          Data Directory for k0s (default: /var/lib/k0s). DO NOT CHANGE for an existing setup, things will break!
  -d, --debug                    Debug logging (default: false)
      --debugListenOn string     Http listenOn for debug pprof handler (default ":6060")
  -l, --logging stringToString   Logging Levels for the different components (default [konnectivity-server=1,kube-apiserver=1,kube-controller-manager=1,kube-scheduler=1,kubelet=1,kube-proxy=1,etcd=info,containerd=info])
```

### SEE ALSO

* [k0s](k0s.md) - k0s - Zero Friction Kubernetes
