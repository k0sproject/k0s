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
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"

	"github.com/k0sproject/k0s/pkg/config"

	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/component-base/logs"
	kubectl "k8s.io/kubectl/pkg/cmd"
	"k8s.io/kubectl/pkg/cmd/plugin"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

func checkKubectlInPath() {
	// exec.LookPath on windows handles filename extensions
	if _, err := exec.LookPath("kubectl"); err == nil {
		return
	}
	exe, err := os.Executable()
	if err != nil {
		return
	}
	switch runtime.GOOS {
	case "windows":
		logrus.Warnf("Kubectl not found in %%PATH%%. Some kubectl plugins try to call it via 'kubectl'. You can use k0s as a drop-in replacement by creating a symlink, for example: `mklink \"%s\" \"%s\\kubectl.exe\"`", exe, filepath.Base(exe))
	default:
		logrus.Warnf("Kubectl not found in $PATH. Some kubectl plugins try to call it via 'kubectl'. You can use k0s as a drop-in replacement by creating a symlink, for example: `sudo ln -s \"%s\" /usr/local/bin/kubectl`", exe)
	}
}

type kubectlPluginHandler struct {
	kubectl.DefaultPluginHandler
}

func (h *kubectlPluginHandler) Execute(executablePath string, cmdArgs, environment []string) error {
	checkKubectlInPath()

	// this will replace the current process and exit on its own if successful
	// error from here is a failure to exec, not an error-exit of a plugin.
	return h.DefaultPluginHandler.Execute(executablePath, cmdArgs, environment)
}

func NewK0sKubectlCmd() *cobra.Command {
	_ = pflag.CommandLine.MarkHidden("log-flush-frequency")
	_ = pflag.CommandLine.MarkHidden("version")

	var idx int
	for i, arg := range os.Args {
		if arg == "kubectl" || arg == "kc" {
			idx = i
			break
		}
	}

	args := kubectl.KubectlOptions{
		IOStreams: genericclioptions.IOStreams{
			In:     os.Stdin,
			Out:    os.Stdout,
			ErrOut: os.Stderr,
		},
		Arguments: os.Args[idx:],
		PluginHandler: &kubectlPluginHandler{
			DefaultPluginHandler: kubectl.DefaultPluginHandler{
				ValidPrefixes: plugin.ValidPluginFilenamePrefixes,
			},
		},
	}

	cmd := kubectl.NewDefaultKubectlCommandWithArgs(args)
	cmd.Aliases = []string{"kc"}

	// Get handle on the original kubectl prerun so we can call it later
	originalPreRunE := cmd.PersistentPreRunE
	cmd.PersistentPreRunE = func(cmd *cobra.Command, args []string) error {
		if err := fallbackToK0sKubeconfig(cmd); err != nil {
			return err
		}

		if err := config.CallParentPersistentPreRun(cmd, args); err != nil {
			return err
		}

		return originalPreRunE(cmd, args)
	}

	cmd.PersistentFlags().AddFlagSet(config.GetKubeCtlFlagSet())

	originalRun := cmd.Run
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		if len(args) > 0 {
			if err := kubectl.HandlePluginCommand(&kubectlPluginHandler{}, args); err != nil {
				// note: the plugin exec will replace the k0s process and exit on it's own,
				// the error here is a failure to exec, not the error-exit of the plugin.
				return fmt.Errorf("kubectl plugin handler failed: %w", err)
			}
		}

		originalRun(cmd, args)

		return nil
	}

	logs.AddFlags(cmd.PersistentFlags())

	return cmd
}

func fallbackToK0sKubeconfig(cmd *cobra.Command) error {
	kubeconfigFlag := cmd.Flags().Lookup("kubeconfig")
	if kubeconfigFlag == nil {
		return fmt.Errorf("kubeconfig flag not found")
	}

	if kubeconfigFlag.Changed {
		// prioritize flag over env
		_ = os.Unsetenv("KUBECONFIG")
		return nil
	}

	if _, ok := os.LookupEnv("KUBECONFIG"); ok {
		return nil
	}

	kubeconfig := config.GetCmdOpts().K0sVars.AdminKubeConfigPath

	// verify that k0s's kubeconfig is readable before pushing it to the env
	if _, err := os.Stat(kubeconfig); err != nil {
		return fmt.Errorf("cannot stat k0s kubeconfig, is the server running?: %w", err)
	}

	if err := kubeconfigFlag.Value.Set(kubeconfig); err != nil {
		return fmt.Errorf("failed to set kubeconfig flag: %w", err)
	}
	return nil
}
