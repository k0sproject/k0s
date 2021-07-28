/*
Copyright 2021 k0s authors

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
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/cobra/doc"
	"github.com/spf13/viper"
	"sigs.k8s.io/yaml"

	"github.com/k0sproject/k0s/cmd/airgap"
	"github.com/k0sproject/k0s/cmd/api"
	"github.com/k0sproject/k0s/cmd/backup"
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
	"github.com/k0sproject/k0s/pkg/apis/k0s.k0sproject.io/v1beta1"
	"github.com/k0sproject/k0s/pkg/build"
	"github.com/k0sproject/k0s/pkg/config"
)

var longDesc string

type cliOpts config.CLIOptions

func NewRootCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "k0s",
		Short: "k0s - Zero Friction Kubernetes",
		PersistentPreRun: func(cmd *cobra.Command, args []string) {
			c := cliOpts(config.GetCmdOpts())
			// set DEBUG from env, or from command flag
			if viper.GetString("debug") != "" || c.Debug {
				logrus.SetLevel(logrus.DebugLevel)
				go func() {
					log.Println("starting debug server under", c.DebugListenOn)
					log.Println(http.ListenAndServe(c.DebugListenOn, nil))
				}()
			}
		},
	}

	cmd.AddCommand(airgap.NewAirgapCmd())
	cmd.AddCommand(api.NewAPICmd())
	cmd.AddCommand(backup.NewBackupCmd())
	cmd.AddCommand(controller.NewControllerCmd())
	cmd.AddCommand(ctr.NewCtrCommand())
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
	cmd.AddCommand(validate.NewValidateCmd())
	cmd.AddCommand(version.NewVersionCmd())
	cmd.AddCommand(worker.NewWorkerCmd())

	cmd.AddCommand(newCompletionCmd())
	cmd.AddCommand(newDefaultConfigCmd())
	cmd.AddCommand(newDocsCmd())

	cmd.DisableAutoGenTag = true
	longDesc = "k0s - The zero friction Kubernetes - https://k0sproject.io"
	if build.EulaNotice != "" {
		longDesc = longDesc + "\n" + build.EulaNotice
	}
	cmd.Long = longDesc

	// workaround for the data-dir location input for the kubectl command
	cmd.PersistentFlags().AddFlagSet(config.GetKubeCtlFlagSet())

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
	cmd := &cobra.Command{
		Use:   "default-config",
		Short: "Output the default k0s configuration yaml to stdout",
		RunE: func(cmd *cobra.Command, args []string) error {
			c := cliOpts(config.GetCmdOpts())
			if err := c.buildConfig(); err != nil {
				return err
			}
			return nil
		},
	}
	cmd.PersistentFlags().AddFlagSet(config.GetPersistentFlagSet())
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
			switch args[0] {
			case "bash":
				return cmd.Root().GenBashCompletion(os.Stdout)
			case "zsh":
				return cmd.Root().GenZshCompletion(os.Stdout)
			case "fish":
				return cmd.Root().GenFishCompletion(os.Stdout, true)
			case "powershell":
				return cmd.Root().GenPowerShellCompletion(os.Stdout)
			}
			return nil
		},
	}
}

func (c *cliOpts) buildConfig() error {
	conf, _ := yaml.Marshal(v1beta1.DefaultClusterConfig(config.DataDir))
	fmt.Print(string(conf))
	return nil
}

func Execute() {
	// just a hack to trick linter which requires to check for errors
	// cobra itself already prints out all errors that happen in subcommands
	err := NewRootCmd().Execute()
	if err != nil {
		log.Fatal(err)
	}
}
