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

	"github.com/spf13/pflag"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/cobra/doc"
	"github.com/spf13/viper"
	"gopkg.in/yaml.v2"

	"github.com/k0sproject/k0s/cmd/api"
	"github.com/k0sproject/k0s/cmd/controller"
	"github.com/k0sproject/k0s/cmd/etcd"
	"github.com/k0sproject/k0s/cmd/install"
	"github.com/k0sproject/k0s/cmd/kubeconfig"
	"github.com/k0sproject/k0s/cmd/kubectl"
	"github.com/k0sproject/k0s/cmd/reset"
	"github.com/k0sproject/k0s/cmd/status"
	"github.com/k0sproject/k0s/cmd/token"
	"github.com/k0sproject/k0s/cmd/validate"
	"github.com/k0sproject/k0s/cmd/worker"
	"github.com/k0sproject/k0s/pkg/apis/v1beta1"
	"github.com/k0sproject/k0s/pkg/build"
	"github.com/k0sproject/k0s/pkg/constant"
)

var (
	cfgFile       string
	dataDir       string
	debug         bool
	debugListenOn string
	k0sVars       constant.CfgVars
	longDesc      string
)

func NewRootCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "k0s",
		Short: "k0s - Zero Friction Kubernetes",
		PersistentPreRun: func(cmd *cobra.Command, args []string) {
			// set DEBUG from env, or from command flag
			if viper.GetString("debug") != "" || debug {
				logrus.SetLevel(logrus.DebugLevel)
				go func() {
					log.Println("starting debug server under", debugListenOn)
					log.Println(http.ListenAndServe(debugListenOn, nil))
				}()
			}
		},
	}
	cmd.AddCommand(api.NewAPICmd())
	cmd.AddCommand(controller.NewControllerCmd())
	cmd.AddCommand(etcd.NewEtcdCmd())
	cmd.AddCommand(install.NewInstallCmd())
	cmd.AddCommand(token.NewTokenCmd())
	cmd.AddCommand(worker.NewWorkerCmd())
	cmd.AddCommand(reset.NewResetCmd())
	cmd.AddCommand(status.NewStatusCmd())
	cmd.AddCommand(validate.NewValidateCmd())
	cmd.AddCommand(kubeconfig.NewKubeConfigCmd())
	cmd.AddCommand(kubectl.NewK0sKubectlCmd())

	cmd.AddCommand(newVersionCmd())
	cmd.AddCommand(newDocsCmd())
	cmd.AddCommand(newDefaultConfigCmd())
	cmd.AddCommand(newCompletionCmd())

	cmd.DisableAutoGenTag = true
	longDesc = "k0s - The zero friction Kubernetes - https://k0sproject.io"
	if build.EulaNotice != "" {
		longDesc = longDesc + "\n" + build.EulaNotice
	}
	cmd.Long = longDesc
	cmd.PersistentFlags().AddFlagSet(getPersistentFlagSet())
	return cmd
}

func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print the k0s version",

		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println(build.Version)
		},
	}
}

func newDocsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "docs",
		Short: "Generate Markdown docs for the k0s binary",
		RunE: func(cmd *cobra.Command, args []string) error {
			err := generateDocs()
			if err != nil {
				return err
			}
			return nil
		},
	}
}

func newDefaultConfigCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "default-config",
		Short: "Output the default k0s configuration yaml to stdout",
		RunE: func(cmd *cobra.Command, args []string) error {
			k0sVars = constant.GetConfig(dataDir)
			if err := buildConfig(); err != nil {
				return err
			}
			return nil
		},
	}
	cmd.PersistentFlags().AddFlagSet(getPersistentFlagSet())
	return cmd
}

func newCompletionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "completion [bash|zsh|fish|powershell]",
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

func buildConfig() error {
	conf, _ := yaml.Marshal(v1beta1.DefaultClusterConfig(k0sVars))
	fmt.Print(string(conf))
	return nil
}

func generateDocs() error {
	if err := doc.GenMarkdownTree(NewRootCmd(), "./docs/cli"); err != nil {
		return err
	}
	return nil
}

func getPersistentFlagSet() *pflag.FlagSet {
	flagset := &pflag.FlagSet{}
	flagset.StringVarP(&cfgFile, "config", "c", "", "config file (default: ./k0s.yaml)")
	flagset.BoolVarP(&debug, "debug", "d", false, "Debug logging (default: false)")
	flagset.StringVar(&dataDir, "data-dir", "", "Data Directory for k0s (default: /var/lib/k0s). DO NOT CHANGE for an existing setup, things will break!")
	flagset.StringVar(&debugListenOn, "debugListenOn", ":6060", "Http listenOn for debug pprof handler")
	return flagset
}

func Execute() {
	// just a hack to trick linter which requires to check for errors
	// cobra itself already prints out all errors that happen in subcommands
	err := NewRootCmd().Execute()
	if err != nil {
		log.Fatal(err)
	}
}
