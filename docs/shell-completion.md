# Enabling Shell Completion

Generate the k0s completion script using the `k0s completion <shell_name>` command, for Bash, Zsh, fish, or PowerShell.

Sourcing the completion script in your shell enables k0s autocompletion.

## Bash

```shell
echo 'source <(k0s completion bash)' >>~/.bashrc
```

To load completions for each session, execute once:

```shell
k0s completion bash > /etc/bash_completion.d/k0s
```

## Zsh

If shell completion is not already enabled in Zsh environment you will need to enable it:

```shell
echo "autoload -U compinit; compinit" >> ~/.zshrc
```

To load completions for each session, execute once:

```shell
k0s completion zsh > "${fpath[1]}/_k0s"
```

**Note**: You must start a new shell for the setup to take effect.

## Fish

```shell
k0s completion fish | source
```

To load completions for each session, execute once:

```shell
k0s completion fish > ~/.config/fish/completions/k0s.fish
```
