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
	"slices"
	"strings"

	"github.com/k0sproject/k0s/pkg/config"

	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/component-base/logs"
	kubectl "k8s.io/kubectl/pkg/cmd"
	"k8s.io/kubectl/pkg/cmd/plugin"
	kubectlutil "k8s.io/kubectl/pkg/cmd/util"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
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
	// Create a new kubectl command without a plugin handler.
	kubectlCmd := kubectl.NewKubectlCommand(kubectl.KubectlOptions{
		IOStreams: genericclioptions.IOStreams{
			In:     os.Stdin,
			Out:    os.Stdout,
			ErrOut: os.Stderr,
		},
	})
	kubectlCmd.Aliases = []string{"kc"}

	// Add some additional kubectl flags:
	persistentFlags := kubectlCmd.PersistentFlags()
	logs.AddFlags(persistentFlags)                         // This is done by k8s.io/component-base/cli
	persistentFlags.AddFlagSet(config.GetKubeCtlFlagSet()) // This is k0s specific

	hookKubectlPluginHandler(kubectlCmd)
	patchPluginListSubcommand(kubectlCmd)

	return kubectlCmd
}

// hookKubectlPluginHandler patches the kubectl command in a way that it will
// execute kubectl's plugin handler before actually executing the command.
func hookKubectlPluginHandler(kubectlCmd *cobra.Command) {
	// Intercept kubectl's flag error func, so that kubectl plugins may be
	// handled properly, e.g. so that `k0s kc foo --bar` works as expected when
	// there's a `kubectl-foo` plugin installed.
	originalFlagErrFunc := kubectlCmd.FlagErrorFunc()
	kubectlCmd.SetFlagErrorFunc(func(cmd *cobra.Command, err error) error {
		handleKubectlPlugins(kubectlCmd)
		return originalFlagErrFunc(cmd, err)
	})

	// Intercept kubectl's PreRunE, so that generic k0s flags are honored and
	// kubectl plugins may be handled properly, e.g. so that `k0s kc foo bar`
	// works as expected when there's a `kubectl-foo` plugin installed.
	originalPreRunE := kubectlCmd.PersistentPreRunE
	kubectlCmd.PersistentPreRunE = func(cmd *cobra.Command, args []string) error {
		handleKubectlPlugins(kubectlCmd)

		// The basic kubectl command will never accept any arguments. This will
		// be handled more or less automatically by Cobra when used as a root
		// command. When run as a subcommand, Cobra will instead pass all of the
		// arguments to the command's run methods, which will trigger some
		// unexpected output. Address this by specifying NoArgs for the kubectl
		// command.
		if cmd == kubectlCmd {
			if err := cobra.NoArgs(cmd, args); err != nil {
				return err
			}
		}

		// In vanilla kubectl, log initialization and flushing is handled by
		// k8s.io/component-base/cli. But k0s doesn't use it, so it needs to
		// deal with that manually.
		logs.InitLogs()
		cobra.OnFinalize(logs.FlushLogs)

		if err := config.CallParentPersistentPreRun(kubectlCmd, args); err != nil {
			return err
		}

		if err := fallbackToK0sKubeconfig(cmd); err != nil {
			return err
		}

		return originalPreRunE(cmd, args)
	}
}

// handleKubectlPlugins calls kubectl's plugin handler and execs the plugin
// without returning if there's any plugin available that handles the given
// command line arguments. Will simply return otherwise.
func handleKubectlPlugins(kubectlCmd *cobra.Command) {
	// Check how the kubectl command has been called on the command line.
	calledAs := kubectlCmd.CalledAs()
	if calledAs == "" {
		return
	}

	// Find the first occurrence of the kubectl command on the command line.
	argOffset := slices.Index(os.Args, calledAs)
	if argOffset < 0 {
		return
	}

	_ = kubectl.NewDefaultKubectlCommandWithArgs(kubectl.KubectlOptions{
		IOStreams: genericclioptions.IOStreams{
			In:     kubectlCmd.InOrStdin(),
			Out:    kubectlCmd.OutOrStdout(),
			ErrOut: kubectlCmd.ErrOrStderr(),
		},
		Arguments: os.Args[argOffset:],
		PluginHandler: &kubectlPluginHandler{
			kubectl.DefaultPluginHandler{
				ValidPrefixes: plugin.ValidPluginFilenamePrefixes,
			},
		},
	})
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

	opts, err := config.GetCmdOpts(cmd)
	if err != nil {
		return err
	}
	kubeconfig := opts.K0sVars.AdminKubeConfigPath

	// verify that k0s's kubeconfig is readable before pushing it to the env
	if _, err := os.Stat(kubeconfig); err != nil {
		return fmt.Errorf("cannot stat k0s kubeconfig, is the server running?: %w", err)
	}

	if err := kubeconfigFlag.Value.Set(kubeconfig); err != nil {
		return fmt.Errorf("failed to set kubeconfig flag: %w", err)
	}
	return nil
}

// patchPluginListSubcommand patches kubectl's "plugin list" command in a way
// that it will look at the kubectl command, not at the k0s command for
// detecting shadowed commands. Kubectl's current implementation of that command
// looks at the root command for detecting collisions. In case of k0s, which
// embeds kubectl as a subcommand rather than at the top level, this means that,
// instead of looking at kubectl command itself, the logic would look at k0s and
// produce the wrong output.
func patchPluginListSubcommand(kubectlCmd *cobra.Command) {
	cmd, _, err := kubectlCmd.Find([]string{"plugin", "list"})
	kubectlutil.CheckErr(err)

	originalRun := cmd.Run
	cmd.Run = func(cmd *cobra.Command, args []string) {
		// Create a dummy kubectl command to be passed as the root command and
		// to be used for command lookups.
		root := kubectl.NewKubectlCommand(kubectl.KubectlOptions{})
		// Best effort of faking the command name. Cobra will split this on
		// spaces, hence use dashes instead. This won't be absolutely correct,
		// but good enough for error reporting on stderr.
		root.Use = strings.ReplaceAll(kubectlCmd.CommandPath(), " ", "-")
		originalRun(root, args)
	}
}
