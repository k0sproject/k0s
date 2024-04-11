/*
Copyright 2020 k0s authors

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package cmd

import (
	"errors"
	"fmt"
	"net/http"
	"os"

	"github.com/k0sproject/k0s/cmd/airgap"
	"github.com/k0sproject/k0s/cmd/api"
	"github.com/k0sproject/k0s/cmd/backup"
	configcmd "github.com/k0sproject/k0s/cmd/config"
	"github.com/k0sproject/k0s/cmd/controller"
	"github.com/k0sproject/k0s/cmd/ctr"
	"github.com/k0sproject/k0s/cmd/etcd"
	"github.com/k0sproject/k0s/cmd/install"
	"github.com/k0sproject/k0s/cmd/kubeconfig"
	"github.com/k0sproject/k0s/cmd/kubectl"
	"github.com/k0sproject/k0s/cmd/reset"
	"github.com/k0sproject/k0s/cmd/restore"
	"github.com/k0sproject/k0s/cmd/start"
	"github.com/k0sproject/k0s/cmd/status"
	"github.com/k0sproject/k0s/cmd/stop"
	"github.com/k0sproject/k0s/cmd/sysinfo"
	"github.com/k0sproject/k0s/cmd/token"
	"github.com/k0sproject/k0s/cmd/validate"
	"github.com/k0sproject/k0s/cmd/version"
	"github.com/k0sproject/k0s/cmd/worker"
	k0slog "github.com/k0sproject/k0s/internal/pkg/log"
	"github.com/k0sproject/k0s/pkg/build"
	"github.com/k0sproject/k0s/pkg/config"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/cobra/doc"
)

func NewRootCmd() *cobra.Command {
	var longDesc string

	cmd := &cobra.Command{
		Use:          "k0s",
		Short:        "k0s - Zero Friction Kubernetes",
		SilenceUsage: true,

		PersistentPreRun: func(cmd *cobra.Command, args []string) {
			if config.Verbose {
				k0slog.SetInfoLevel()
			}

			if config.Debug {
				// TODO: check if it actually works and is not overwritten by something else
				k0slog.SetDebugLevel()

				go func() {
					log := logrus.WithField("debug_server", config.DebugListenOn)
					log.Debug("Starting debug server")
					if err := http.ListenAndServe(config.DebugListenOn, nil); !errors.Is(err, http.ErrServerClosed) {
						log.WithError(err).Debug("Failed to start debug server")
					} else {
						log.Debug("Debug server closed")
					}
				}()
			}
		},
	}

	cmd.AddCommand(airgap.NewAirgapCmd())
	cmd.AddCommand(api.NewAPICmd())
	cmd.AddCommand(backup.NewBackupCmd())
	cmd.AddCommand(controller.NewControllerCmd())
	cmd.AddCommand(ctr.NewCtrCommand())
	cmd.AddCommand(configcmd.NewConfigCmd())
	cmd.AddCommand(etcd.NewEtcdCmd())
	cmd.AddCommand(install.NewInstallCmd())
	cmd.AddCommand(kubeconfig.NewKubeConfigCmd())
	cmd.AddCommand(kubectl.NewK0sKubectlCmd())
	cmd.AddCommand(reset.NewResetCmd())
	cmd.AddCommand(restore.NewRestoreCmd())
	cmd.AddCommand(start.NewStartCmd())
	cmd.AddCommand(status.NewStatusCmd())
	cmd.AddCommand(stop.NewStopCmd())
	cmd.AddCommand(sysinfo.NewSysinfoCmd())
	cmd.AddCommand(token.NewTokenCmd())
	cmd.AddCommand(validate.NewValidateCmd()) // hidden+deprecated
	cmd.AddCommand(version.NewVersionCmd())
	cmd.AddCommand(worker.NewWorkerCmd())

	cmd.AddCommand(newCompletionCmd())
	cmd.AddCommand(newDefaultConfigCmd()) // hidden+deprecated
	cmd.AddCommand(newDocsCmd())

	cmd.DisableAutoGenTag = true
	longDesc = "k0s - The zero friction Kubernetes - https://k0sproject.io"
	if build.EulaNotice != "" {
		longDesc = longDesc + "\n" + build.EulaNotice
	}
	cmd.Long = longDesc
	return cmd
}

func newDocsCmd() *cobra.Command {
	return &cobra.Command{
		Use:       "docs <markdown|man>",
		Short:     "Generate k0s command documentation",
		ValidArgs: []string{"markdown", "man"},
		Args:      cobra.ExactValidArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			switch args[0] {
			case "markdown":
				return doc.GenMarkdownTree(NewRootCmd(), "./docs/cli")
			case "man":
				return doc.GenManTree(NewRootCmd(), &doc.GenManHeader{Title: "k0s", Section: "1"}, "./man")
			}
			return fmt.Errorf("invalid format")
		},
	}
}

func newDefaultConfigCmd() *cobra.Command {
	cmd := configcmd.NewCreateCmd()
	cmd.Hidden = true
	cmd.Deprecated = "use 'k0s config create' instead"
	cmd.Use = "default-config"
	return cmd
}

func newCompletionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "completion <bash|zsh|fish|powershell>",
		Short: "Generate completion script",
		Long: `To load completions:

Bash:

$ source <(k0s completion bash)

# To load completions for each session, execute once:
  $ k0s completion bash > /etc/bash_completion.d/k0s

Zsh:

# If shell completion is not already enabled in your environment you will need
# to enable it.  You can execute the following once:

$ echo "autoload -U compinit; compinit" >> ~/.zshrc

# To load completions for each session, execute once:
$ k0s completion zsh > "${fpath[1]}/_k0s"

# You will need to start a new shell for this setup to take effect.

Fish:

$ k0s completion fish | source

# To load completions for each session, execute once:
$ k0s completion fish > ~/.config/fish/completions/k0s.fish
`,
		DisableFlagsInUseLine: true,
		ValidArgs:             []string{"bash", "zsh", "fish", "powershell"},
		Args:                  cobra.ExactValidArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			out := cmd.OutOrStdout()
			switch args[0] {
			case "bash":
				return cmd.Root().GenBashCompletion(out)
			case "zsh":
				return cmd.Root().GenZshCompletion(out)
			case "fish":
				return cmd.Root().GenFishCompletion(out, true)
			case "powershell":
				return cmd.Root().GenPowerShellCompletion(out)
			}
			return nil
		},
	}
}

func Execute() {
	if err := NewRootCmd().Execute(); err != nil {
		os.Exit(1)
	}
}
