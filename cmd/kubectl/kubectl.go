/*
Copyright 2022 k0s authors

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
	"runtime"
	"syscall"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	kubectl "k8s.io/kubectl/pkg/cmd"

	"github.com/k0sproject/k0s/pkg/config"
)

type CmdOpts config.CLIOptions

type kubectlPluginHandler struct{}

func (h *kubectlPluginHandler) Lookup(filename string) (string, bool) {
	path, err := exec.LookPath(fmt.Sprintf("kubectl-%s", filename))
	if err != nil || path == "" {
		return "", false
	}
	return path, true
}

// adapted from kubectl.DefaultPluginHandler
func (h *kubectlPluginHandler) Execute(executablePath string, cmdArgs, environment []string) error {
	if _, err := exec.LookPath("kubectl"); err != nil {
		if exe, err := os.Executable(); err == nil {
			logrus.Warnf("kubectl not found in $PATH. many kubectl plugins try to run 'kubectl'. you can use k0s as a replacement by creating a symlink, for example: `sudo ln -s \"%s\" /usr/local/bin/kubectl`", exe)
		}
	}

	// Windows does not support exec syscall.
	if runtime.GOOS == "windows" {
		cmd := exec.Command(executablePath, cmdArgs...)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		cmd.Stdin = os.Stdin
		cmd.Env = environment
		if err := cmd.Run(); err != nil {
			return err
		}
		os.Exit(0)
	}

	// invoke cmd binary relaying the environment and args given
	// append executablePath to cmdArgs, as execve will make first argument the "binary name".
	return syscall.Exec(executablePath, append([]string{executablePath}, cmdArgs...), environment)
}

func NewK0sKubectlCmd() *cobra.Command {
	_ = pflag.CommandLine.MarkHidden("log-flush-frequency")
	_ = pflag.CommandLine.MarkHidden("version")

	args := kubectl.KubectlOptions{
		IOStreams: genericclioptions.IOStreams{
			In:     os.Stdin,
			Out:    os.Stdout,
			ErrOut: os.Stderr,
		},
		Arguments:     os.Args,
		PluginHandler: &kubectlPluginHandler{},
	}
	cmd := kubectl.NewKubectlCommand(args)

	cmd.Aliases = []string{"kc"}
	// Get handle on the original kubectl prerun so we can call it later
	originalPreRunE := cmd.PersistentPreRunE
	cmd.PersistentPreRunE = func(cmd *cobra.Command, args []string) error {
		// Call parents pre-run if exists, cobra does not do this automatically
		// See: https://github.com/spf13/cobra/issues/216
		if parent := cmd.Parent(); parent != nil {
			if parent.PersistentPreRun != nil {
				parent.PersistentPreRun(parent, args)
			}
			if parent.PersistentPreRunE != nil {
				err := parent.PersistentPreRunE(parent, args)
				if err != nil {
					return err
				}
			}
		}
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

	originalRun := cmd.Run
	cmd.Run = func(cmd *cobra.Command, args []string) {
		if len(args) > 0 {
			if err := kubectl.HandlePluginCommand(&kubectlPluginHandler{}, args); err != nil {
				// note: the plugin exec will replace the k0s process and exit on it's own,
				// the error here is a failure to exec, not the error-exit of the plugin.
				logrus.Fatalf("kubectl plugin handler failed: %v", err)
			}
		}

		originalRun(cmd, args)
	}

	return cmd
}
