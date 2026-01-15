// SPDX-FileCopyrightText: 2021 k0s authors
// SPDX-License-Identifier: Apache-2.0

package status

import (
	"encoding/json"
	"fmt"
	"io"
	"runtime"

	"github.com/k0sproject/k0s/cmd/internal"
	"github.com/k0sproject/k0s/pkg/component/status"
	"github.com/k0sproject/k0s/pkg/config"

	"github.com/spf13/cobra"
	"sigs.k8s.io/yaml"
)

func NewStatusCmd() *cobra.Command {
	var (
		debugFlags internal.DebugFlags
		output     string
	)

	cmd := &cobra.Command{
		Use:              "status",
		Short:            "Get k0s instance status information",
		Example:          `The command will return information about system init, PID, k0s role, kubeconfig and similar.`,
		PersistentPreRun: debugFlags.Run,
		Args:             cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			opts, err := config.GetCmdOpts(cmd)
			if err != nil {
				return err
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

	pflags := cmd.PersistentFlags()
	debugFlags.AddToFlagSet(pflags)
	var socketDefault string
	if runtime.GOOS == "windows" {
		socketDefault = status.DefaultSocketPath
	} else {
		socketDefault = "<rundir>/status.sock"
	}

	pflags.String("status-socket", "", "Full path to the k0s status socket (default: "+socketDefault+")")

	cmd.AddCommand(NewStatusSubCmdComponents())

	cmd.Flags().StringVarP(&output, "out", "o", "", "sets type of output to json or yaml")

	return cmd
}

func NewStatusSubCmdComponents() *cobra.Command {
	var maxCount int

	cmd := &cobra.Command{
		Use:     "components",
		Short:   "Get k0s instance component status information",
		Example: `The command will return information about k0s components.`,
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			opts, err := config.GetCmdOpts(cmd)
			if err != nil {
				return err
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

	flags := cmd.Flags()
	flags.IntVar(&maxCount, "max-count", 1, "how many latest probes to show")
	flags.StringP("out", "o", "", "")
	outFlag := flags.Lookup("out")
	outFlag.Hidden = true
	outFlag.Deprecated = "it has no effect and will be removed in a future release"

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
		if status.CNI != nil {
			fmt.Fprintln(w, "\nCNI Status:")
			fmt.Fprintln(w, " Provider:", status.CNI.Provider)
			fmt.Fprintln(w, " Health:", status.CNI.Health)

			if len(status.CNI.Components) > 0 {
				fmt.Fprintln(w, " Components:")
				for _, c := range status.CNI.Components {
					fmt.Fprintf(w, "   - %s\n", c)
				}
			}

			if status.CNI.Error != "" {
				fmt.Fprintln(w, "Error:", status.CNI.Error)
			}
		}

	}
}
