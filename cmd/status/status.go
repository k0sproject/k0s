/*
Copyright 2023 k0s authors

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
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
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
			cmd.SilenceUsage = true
			if runtime.GOOS == "windows" {
				return fmt.Errorf("currently not supported on windows")
			}

			statusInfo, err := status.GetStatusInfo(config.StatusSocket)
			if err != nil {
				return err
			}
			if statusInfo != nil {
				printStatus(cmd.OutOrStdout(), statusInfo, output)
			} else {
				fmt.Fprintln(cmd.OutOrStdout(), "K0s is not running")
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
			cmd.SilenceUsage = true
			if runtime.GOOS == "windows" {
				return fmt.Errorf("currently not supported on windows")
			}
			fmt.Fprintln(os.Stderr, "!!! per component status is not yet finally ready, information here might be not full yet")
			state, err := status.GetComponentStatus(config.StatusSocket, maxCount)
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

// TODO: move it somewhere else, now here just for quick manual testing
func GetOverSocket(socketPath string, path string, tgt interface{}) error {

	httpc := http.Client{
		Transport: &http.Transport{
			DialContext: func(_ context.Context, _, _ string) (net.Conn, error) {
				return net.Dial("unix", socketPath)
			},
		},
	}

	response, err := httpc.Get("http://localhost/" + path)
	if err != nil {
		return err
	}
	defer response.Body.Close()

	responseData, err := io.ReadAll(response.Body)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(responseData, tgt); err != nil {
		return err
	}

	return nil
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
