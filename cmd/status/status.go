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
package status

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"runtime"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v2"

	"github.com/k0sproject/k0s/pkg/config"
	"github.com/k0sproject/k0s/pkg/install"
)

type CmdOpts config.CLIOptions

var (
	output string
)

func NewStatusCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "status",
		Short:   "Get k0s instance status information",
		Example: `The command will return information about system init, PID, k0s role, kubeconfig and similar.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			cmd.SilenceUsage = true
			if runtime.GOOS == "windows" {
				return fmt.Errorf("currently not supported on windows")
			}

			statusInfo, err := install.GetStatusInfo(config.StatusSocket)
			if err != nil {
				return err
			}
			if statusInfo != nil {
				printStatus(statusInfo, output)
			} else {
				fmt.Println("K0s is not running")
			}
			return nil
		},
	}

	cmd.SilenceUsage = true
	cmd.PersistentFlags().StringVarP(&output, "out", "o", "", "sets type of output to json or yaml")
	cmd.PersistentFlags().StringVar(&config.StatusSocket, "status-socket", filepath.Join(config.K0sVars.RunDir, "status.sock"), "Full file path to the socket file.")

	return cmd
}

func printStatus(status *install.K0sStatus, output string) {
	switch output {
	case "json":
		jsn, _ := json.MarshalIndent(status, "", "   ")
		fmt.Println(string(jsn))
	case "yaml":
		ym, _ := yaml.Marshal(status)
		fmt.Println(string(ym))
	default:
		fmt.Println("Version:", status.Version)
		fmt.Println("Process ID:", status.Pid)
		fmt.Println("Role:", status.Role)
		fmt.Println("Workloads:", status.Workloads)
		fmt.Println("SingleNode:", status.SingleNode)

		if status.SysInit != "" {
			fmt.Println("Init System:", status.SysInit)
		}
		if status.StubFile != "" {
			fmt.Println("Service file:", status.StubFile)
		}
	}
}
