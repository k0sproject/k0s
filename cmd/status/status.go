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

package status

import (
	"encoding/json"
	"fmt"
	"io"
	"path/filepath"
	"runtime"

	"github.com/k0sproject/k0s/pkg/component/status"
	"github.com/k0sproject/k0s/pkg/config"

	"github.com/spf13/cobra"
	"sigs.k8s.io/yaml"
)

func NewStatusCmd() *cobra.Command {
	var output string
	cmd := &cobra.Command{
		Use:     "status",
		Short:   "Get k0s instance status information",
		Example: `The command will return information about system init, PID, k0s role, kubeconfig and similar.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			opts, err := config.GetCmdOpts(cmd)
			if err != nil {
				return err
			}
			cmd.SilenceUsage = true
			if runtime.GOOS == "windows" {
				return fmt.Errorf("currently not supported on windows")
			}

			statusInfo, err := status.GetStatusInfo(opts.K0sVars.StatusSocketPath)
			if err != nil {
				return err
			}
			if statusInfo != nil {
				printStatus(cmd.OutOrStdout(), statusInfo, output)
			} else {
				return config.ErrK0sNotRunning
			}
			return nil
		},
	}

	cmd.SilenceUsage = true
	cmd.PersistentFlags().StringVarP(&output, "out", "o", "", "sets type of output to json or yaml")
	cmd.PersistentFlags().StringVar(&config.StatusSocket, "status-socket", filepath.Join(config.K0sVars.RunDir, "status.sock"), "Full file path to the socket file.")
	cmd.AddCommand(NewStatusSubCmdComponents())
	return cmd
}

func NewStatusSubCmdComponents() *cobra.Command {
	var maxCount int
	cmd := &cobra.Command{
		Use:     "components",
		Short:   "Get k0s instance component status information",
		Example: `The command will return information about k0s components.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			opts, err := config.GetCmdOpts(cmd)
			if err != nil {
				return err
			}
			cmd.SilenceUsage = true
			if runtime.GOOS == "windows" {
				return fmt.Errorf("currently not supported on windows")
			}
			fmt.Fprintln(cmd.ErrOrStderr(), "!!! per component status is not yet finally ready, information here might be not full yet")
			state, err := status.GetComponentStatus(opts.K0sVars.StatusSocketPath, maxCount)
			if err != nil {
				return err
			}
			d, err := yaml.Marshal(state)
			if err != nil {
				return err
			}
			fmt.Println(string(d))
			return nil
		},
	}
	cmd.Flags().IntVar(&maxCount, "max-count", 1, "how many latest probes to show")
	return cmd

}

func printStatus(w io.Writer, status *status.K0sStatus, output string) {
	switch output {
	case "json":
		jsn, _ := json.MarshalIndent(status, "", "   ")
		fmt.Fprintln(w, string(jsn))
	case "yaml":
		ym, _ := yaml.Marshal(status)
		fmt.Fprintln(w, string(ym))
	default:
		fmt.Fprintln(w, "Version:", status.Version)
		fmt.Fprintln(w, "Process ID:", status.Pid)
		fmt.Fprintln(w, "Role:", status.Role)
		fmt.Fprintln(w, "Workloads:", status.Workloads)
		fmt.Fprintln(w, "SingleNode:", status.SingleNode)
		if status.Workloads {
			fmt.Fprintln(w, "Kube-api probing successful:", status.WorkerToAPIConnectionStatus.Success)
			fmt.Fprintln(w, "Kube-api probing last error: ", status.WorkerToAPIConnectionStatus.Message)
		}
		if status.SysInit != "" {
			fmt.Fprintln(w, "Init System:", status.SysInit)
		}
		if status.StubFile != "" {
			fmt.Fprintln(w, "Service file:", status.StubFile)
		}

	}
}
