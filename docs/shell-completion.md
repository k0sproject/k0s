# Enabling Shell Completion

The k0s completion script for Bash, zsh, fish and powershell can be generated with the command
`k0s completion < shell >`. 

Sourcing the completion script in your shell enables k0s autocompletion.

### Bash

```sh
echo 'source <(k0s completion bash)' >>~/.bashrc
```

```sh
# To load completions for each session, execute once:
$ k0s completion bash > /etc/bash_completion.d/k0s
```
### Zsh

If shell completion is not already enabled in your environment you will need to enable it. You can execute the following once:
```sh
$ echo "autoload -U compinit; compinit" >> ~/.zshrc
```
```sh
# To load completions for each session, execute once:
$ k0s completion zsh > "${fpath[1]}/_k0s"
```
You will need to start a new shell for this setup to take effect.

### Fish

```sh
$ k0s completion fish | source
```
```sh
# To load completions for each session, execute once:
$ k0s completion fish > ~/.config/fish/completions/k0s.fish
```
