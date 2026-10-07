// SPDX-FileCopyrightText: 2020 k0s authors
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/k0sproject/k0s/cmd/airgap"
	"github.com/k0sproject/k0s/cmd/api"
	"github.com/k0sproject/k0s/cmd/config"
	"github.com/k0sproject/k0s/cmd/ctr"
	"github.com/k0sproject/k0s/cmd/etcd"
	"github.com/k0sproject/k0s/cmd/install"
	"github.com/k0sproject/k0s/cmd/keepalived"
	"github.com/k0sproject/k0s/cmd/kubeconfig"
	"github.com/k0sproject/k0s/cmd/kubectl"
	"github.com/k0sproject/k0s/cmd/reset"
	"github.com/k0sproject/k0s/cmd/start"
	"github.com/k0sproject/k0s/cmd/status"
	"github.com/k0sproject/k0s/cmd/stop"
	"github.com/k0sproject/k0s/cmd/sysinfo"
	"github.com/k0sproject/k0s/cmd/token"
	"github.com/k0sproject/k0s/cmd/validate"
	"github.com/k0sproject/k0s/cmd/version"
	"github.com/k0sproject/k0s/cmd/worker"
	"github.com/k0sproject/k0s/internal/schemagen"
	"github.com/k0sproject/k0s/pkg/build"

	"github.com/spf13/cobra"
	"github.com/spf13/cobra/doc"
)

func NewRootCmd() *cobra.Command {
	var longDesc string

	cmd := &cobra.Command{
		Use:          "k0s",
		Short:        "k0s - Zero Friction Kubernetes",
		SilenceUsage: true,
	}

	cmd.AddCommand(airgap.NewAirgapCmd())
	cmd.AddCommand(api.NewAPICmd())
	cmd.AddCommand(ctr.NewCtrCommand())
	cmd.AddCommand(config.NewConfigCmd())
	cmd.AddCommand(etcd.NewEtcdCmd())
	cmd.AddCommand(install.NewInstallCmd())
	cmd.AddCommand(keepalived.NewKeepalivedConfigCmd())
	cmd.AddCommand(kubeconfig.NewKubeConfigCmd())
	cmd.AddCommand(kubectl.NewK0sKubectlCmd())
	cmd.AddCommand(reset.NewResetCmd())
	cmd.AddCommand(start.NewStartCmd())
	cmd.AddCommand(stop.NewStopCmd())
	cmd.AddCommand(status.NewStatusCmd())
	cmd.AddCommand(sysinfo.NewSysinfoCmd())
	cmd.AddCommand(token.NewTokenCmd())
	cmd.AddCommand(validate.NewValidateCmd()) // hidden+deprecated
	cmd.AddCommand(version.NewVersionCmd())
	cmd.AddCommand(worker.NewWorkerCmd())

	cmd.AddCommand(newCompletionCmd())
	cmd.AddCommand(newDefaultConfigCmd()) // hidden+deprecated
	cmd.AddCommand(newDocsCmd())

	addPlatformSpecificCommands(cmd)

	cmd.DisableAutoGenTag = true
	longDesc = "k0s - The zero friction Kubernetes - https://k0sproject.io"
	if build.EulaNotice != "" {
		longDesc = longDesc + "\n" + build.EulaNotice
	}
	cmd.Long = longDesc
	return cmd
}

func newDocsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "docs",
		Short: "Generate k0s command documentation",
		Args:  cobra.NoArgs,
	}
	cmd.AddCommand(
		&cobra.Command{Use: "markdown", Args: cobra.NoArgs, RunE: func(*cobra.Command, []string) error {
			return doc.GenMarkdownTree(NewRootCmd(), "./docs/cli")
		}},
		&cobra.Command{Use: "man", Args: cobra.NoArgs, RunE: func(*cobra.Command, []string) error {
			return doc.GenManTree(NewRootCmd(), &doc.GenManHeader{Title: "k0s", Section: "1"}, "./man")
		}},
		newJSONSchemaDocsCmd(),
	)
	return cmd
}

func newJSONSchemaDocsCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "json-schema", Short: "Generate JSON Schemas for k0s APIs", Args: cobra.NoArgs}
	cmd.AddCommand(
		&cobra.Command{
			Use:   "list",
			Short: "List available JSON Schemas",
			Args:  cobra.NoArgs,
			RunE: func(cmd *cobra.Command, _ []string) error {
				identifiers, err := schemagen.List()
				if err != nil {
					return err
				}
				for _, id := range identifiers {
					if _, err := fmt.Fprintln(cmd.OutOrStdout(), id); err != nil {
						return err
					}
				}
				return nil
			},
		},
		newJSONSchemaGenerateCmd(),
	)
	return cmd
}

func newJSONSchemaGenerateCmd() *cobra.Command {
	var all bool
	var output string
	cmd := &cobra.Command{
		Use:   "gen [apiVersion kind]",
		Short: "Generate JSON Schema",
		Args: func(cmd *cobra.Command, args []string) error {
			if all {
				if output == "" {
					return errors.New("required flag(s) \"output\" not set")
				}
				return cobra.NoArgs(cmd, args)
			}
			if output != "" {
				return errors.New("--output requires --all")
			}
			return cobra.ExactArgs(2)(cmd, args)
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if all {
				return generateAllSchemas(output)
			}
			data, err := schemagen.Generate(args[0], args[1])
			if err != nil {
				return err
			}
			_, err = cmd.OutOrStdout().Write(data)
			return err
		},
	}
	cmd.Flags().BoolVar(&all, "all", false, "generate all schemas")
	cmd.Flags().StringVarP(&output, "output", "o", "", "output directory for --all")
	return cmd
}

func generateAllSchemas(output string) error {
	identifiers, err := schemagen.List()
	if err != nil {
		return err
	}
	for _, id := range identifiers {
		data, err := schemagen.Generate(id.APIVersion, id.Kind)
		if err != nil {
			return err
		}
		group, version, _ := strings.Cut(id.APIVersion, "/")
		file := filepath.Join(output, group, id.Kind+"_"+version+".json")
		if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(file, data, 0o644); err != nil {
			return err
		}
	}
	return nil
}

func newDefaultConfigCmd() *cobra.Command {
	cmd := config.NewCreateCmd()
	cmd.Hidden = true
	cmd.Deprecated = "use 'k0s config create' instead"
	cmd.Use = "default-config"
	return cmd
}

func newCompletionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "completion {bash|zsh|fish|powershell}",
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
		Args:                  cobra.MatchAll(cobra.ExactArgs(1), cobra.OnlyValidArgs),
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
