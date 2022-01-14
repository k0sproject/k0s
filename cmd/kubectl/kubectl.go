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
package kubectl

import (
	"os"

	"github.com/davecgh/go-spew/spew"
	"github.com/k0sproject/k0s/pkg/config"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	kubectl "k8s.io/kubectl/pkg/cmd"
)

type CmdOpts config.CLIOptions

func NewK0sKubectlCmd() *cobra.Command {
	_ = pflag.CommandLine.MarkHidden("log-flush-frequency")
	_ = pflag.CommandLine.MarkHidden("version")

	cmd := kubectl.NewDefaultKubectlCommand()

	cmd.Aliases = []string{"kc"}
	// Get handle on the original kubectl prerun so we can call it later
	originalPreRunE := cmd.PersistentPreRunE
	cmd.PersistentPreRunE = func(cmd *cobra.Command, args []string) error {
		spew.Dump("PRE-RUN ARGS", args)
		// Call parents pre-run if exists, cobra does not do this automatically
		// See: https://github.com/spf13/cobra/issues/216
		// if parent := cmd.Parent(); parent != nil {
		// 	if parent.PersistentPreRun != nil {
		// 		parent.PersistentPreRun(parent, args)
		// 	}
		// 	if parent.PersistentPreRunE != nil {
		// 		err := parent.PersistentPreRunE(parent, args)
		// 		if err != nil {
		// 			return err
		// 		}
		// 	}
		// }
		c := CmdOpts(config.GetCmdOpts())
		if os.Getenv("KUBECONFIG") == "" {
			// Verify we can read the config before pushing it to env
			file, err := os.OpenFile(c.K0sVars.AdminKubeConfigPath, os.O_RDONLY, 0600)
			if err != nil {
				logrus.Errorf("cannot read admin kubeconfig at %s, is the server running?", c.K0sVars.AdminKubeConfigPath)
				return err
			}
			defer file.Close()
			os.Setenv("KUBECONFIG", c.K0sVars.AdminKubeConfigPath)
		}

		return originalPreRunE(cmd, args)
	}

	return cmd
}
