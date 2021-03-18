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
	"os"
	"path/filepath"
	"time"

	"github.com/olekukonko/tablewriter"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/util/retry"

	"github.com/k0sproject/k0s/pkg/token"
)

func init() {
	tokenCmd.Flags().StringVar(&kubeConfig, "kubeconfig", k0sVars.AdminKubeConfigPath, "path to kubeconfig file [$KUBECONFIG]")
	if kubeConfig == "" {
		kubeConfig = viper.GetString("KUBECONFIG")
	}

	tokenCmd.AddCommand(tokenCreateCmd)
	tokenCmd.AddCommand(tokenListCmd)
	tokenCmd.AddCommand(tokenInvalidateCmd)

	tokenCreateCmd.Flags().StringVar(&tokenExpiry, "expiry", "0s", "Expiration time of the token. Format 1.5h, 2h45m or 300ms.")
	tokenCreateCmd.Flags().StringVar(&tokenRole, "role", "worker", "Either worker or controller")
	tokenCreateCmd.Flags().BoolVar(&waitCreate, "wait", false, "wait forever (default false)")

	tokenListCmd.Flags().StringVar(&tokenRole, "role", "", "Either worker,controller or empty for all roles")

	addPersistentFlags(tokenCreateCmd)

	// shell completion options
	_ = tokenCreateCmd.MarkFlagRequired("role")
	_ = tokenCreateCmd.RegisterFlagCompletionFunc("role", func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return []string{"worker", "controller"}, cobra.ShellCompDirectiveDefault
	})

	addPersistentFlags(tokenCmd)
}

var (
	kubeConfig  string
	tokenExpiry string
	tokenRole   string
	waitCreate  bool

	// tokenCmd creates new token management command
	tokenCmd = &cobra.Command{
		Use:   "token",
		Short: "Manage join tokens",
	}
)

var (
	tokenCreateCmd = &cobra.Command{
		Use:   "create",
		Short: "Create join token",
		Example: `k0s token create --role worker --expiry 100h //sets expiration time to 100 hours
k0s token create --role worker --expiry 10m  //sets expiration time to 10 minutes
`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Disable logrus for token commands
			logrus.SetLevel(logrus.FatalLevel)

			clusterConfig, err := ConfigFromYaml(cfgFile)
			if err != nil {
				return err
			}
			expiry, err := time.ParseDuration(tokenExpiry)
			if err != nil {
				return err
			}

			var bootstrapConfig string
			// we will retry every second for two minutes and then error
			err = retry.OnError(wait.Backoff{
				Steps:    120,
				Duration: 1 * time.Second,
				Factor:   1.0,
				Jitter:   0.1,
			}, func(err error) bool {
				return waitCreate
			}, func() error {
				bootstrapConfig, err = createKubeletBootstrapConfig(clusterConfig, tokenRole, expiry)
				return err
			})
			if err != nil {
				return err
			}
			fmt.Println(bootstrapConfig)
			return nil
		},
	}

	tokenInvalidateCmd = &cobra.Command{
		Use:     "invalidate",
		Short:   "Invalidates existing join token",
		Example: "k0s token invalidate xyz123",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) < 1 {
				return fmt.Errorf("invalidate requires at least one token Id to invalidate")
			}
			manager, err := token.NewManager(filepath.Join(k0sVars.AdminKubeConfigPath))
			if err != nil {
				return err
			}

			for _, id := range args {
				err := manager.Remove(id)
				if err != nil {
					return err
				}
				fmt.Printf("token %s deleted succesfully\n", id)
			}
			return nil
		},
	}

	tokenListCmd = &cobra.Command{
		Use:     "list",
		Short:   "List join tokens",
		Example: `k0s token list --role worker // list worker tokens`,
		RunE: func(cmd *cobra.Command, args []string) error {
			manager, err := token.NewManager(filepath.Join(k0sVars.AdminKubeConfigPath))
			if err != nil {
				return err
			}

			tokens, err := manager.List(tokenRole)
			if err != nil {
				return err
			}
			if len(tokens) == 0 {
				fmt.Println("No k0s join tokens found")
				return nil
			}

			//fmt.Printf("Tokens: %v \n", tokens)
			table := tablewriter.NewWriter(os.Stdout)
			table.SetHeader([]string{"ID", "Role", "Expires at"})
			table.SetAutoWrapText(false)
			table.SetAutoFormatHeaders(true)
			table.SetHeaderAlignment(tablewriter.ALIGN_LEFT)
			table.SetAlignment(tablewriter.ALIGN_LEFT)
			table.SetCenterSeparator("")
			table.SetColumnSeparator("")
			table.SetRowSeparator("")
			table.SetHeaderLine(false)
			table.SetBorder(false)
			table.SetTablePadding("\t") // pad with tabs
			table.SetNoWhiteSpace(true)
			for _, t := range tokens {
				table.Append(t.ToArray())
			}
			table.Render()
			return nil
		},
	}
)
