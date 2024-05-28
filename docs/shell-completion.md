# Enabling Shell Completion

## Introduction

Shell completion enhances the user experience by providing auto-completion for
commands in the terminal. K0s supports shell completion for the following
shells:

- `bash`, [GNU Bash](https://www.gnu.org/software/bash/)
- `zsh`, [the Z-shell](https://www.zsh.org/)
- `fish`, [the friendly interactive shell](https://fishshell.com/)
- `powershell`, [Microsoft PowerShell](https://learn.microsoft.com/powershell/)

## General Usage

To generate a completion script for your shell, use the following command: `k0s
completion <shell_name>`. Sourcing the completion script in your shell enables
k0s autocompletion.

## bash

One-shot usage: `source <(k0s completion bash)`.

This is a recipe to load completions for each new shell. Adjust to your personal
needs:

```bash
mkdir ~/.bash_completion.d
k0s completion bash >~/.bash_completion.d/k0s

cat <<'EOF' >~/.bashrc
for compFile in ~/.bash_completion.d/*; do
  [ ! -f "$compFile" ] || source -- "$compFile"
done
unset compFile
EOF
```

Then restart the shell or source `~/.bashrc`.

## zsh

One-shot usage: `source <(k0s completion bash)`.

Following a recipe to load completions for each new shell. Adjust to your
personal needs. If shell completion is not already enabled in your zsh
environment you will need to enable it:

```zsh
echo "autoload -Uz compinit; compinit" >>~/.zshrc
```

Place the completion script in a custom `site-functions` folder:

```zsh
mkdir -p -- ~/.local/share/zsh/site-functions
k0s completion zsh >~/.local/share/zsh/site-functions/_k0s
```

Edit `~/.zshrc` and add the line `fpath+=(~/.local/share/zsh/site-functions)`
somewhere before `compinit` is called. After that, restart the shell.

When using [Oh My ZSH!], you can create a [custom plugin]:

```zsh
mkdir -- "$ZSH_CUSTOM/plugins/k0s"
cat <<'EOF' >"$ZSH_CUSTOM/plugins/k0s/k0s.plugin.zsh"
k0s completion zsh >| "$ZSH_CACHE_DIR/completions/_k0s" &|
EOF
omz plugin enable k0s
```

Then restart the shell.

[Oh My ZSH!]: https://ohmyz.sh/
[custom plugin]: https://github.com/ohmyzsh/ohmyzsh/wiki/Customization#overriding-and-adding-plugins

## fish

One-shot usage: `k0s completion fish | source`.

This is a recipe to load completions for each new shell. Adjust to your personal
needs:

```shell
mkdir -p -- "${XDG_CONFIG_HOME:-$HOME/.config}/fish/completions"
k0s completion fish >"${XDG_CONFIG_HOME:-$HOME/.config}/fish/completions/k0s.fish"
```

Then restart the shell.

## powershell

Save the completion script into a file:

```powershell
k0s completion powershell > C:\path\to\k0s.ps1
```

You can import it like so:

```powershell
Import-Module C:\path\to\k0s.ps1
```

To automatically load the module for each new shell session, add the above line
to your shell profile. You can find the path to your profile via `Write-Output
$profile`.
