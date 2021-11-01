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
package token

import (
	"fmt"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/util/retry"

	"github.com/k0sproject/k0s/pkg/config"
	"github.com/k0sproject/k0s/pkg/token"
)

var createTokenRole string

func tokenCreateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create join token",
		Example: `k0s token create --role worker --expiry 100h //sets expiration time to 100 hours
k0s token create --role worker --expiry 10m  //sets expiration time to 10 minutes
`,
		PreRunE: checkCreateTokenRole,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Disable logrus for token commands
			logrus.SetLevel(logrus.FatalLevel)
			c := CmdOpts(config.GetCmdOpts())
			cfg, err := config.GetNodeConfig(c.CfgFile, c.K0sVarspkg/component/controller/clusterConfig.go
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
				bootstrapConfig, err = token.CreateKubeletBootstrapConfig(cmd.Context(), cfg, c.K0sVars, createTokenRole, expiry)

				return err
			})
			if err != nil {
				return err
			}

			fmt.Println(bootstrapConfig)

			return nil
		},
	}
	// append flags
	cmd.PersistentFlags().AddFlagSet(config.GetPersistentFlagSet())

	cmd.Flags().StringVar(&tokenExpiry, "expiry", "0s", "Expiration time of the token. Format 1.5h, 2h45m or 300ms.")
	cmd.Flags().StringVar(&createTokenRole, "role", "worker", "Either worker or controller")
	cmd.Flags().BoolVar(&waitCreate, "wait", false, "wait forever (default false)")

	return cmd
}

func checkCreateTokenRole(cmd *cobra.Command, args []string) error {
	if createTokenRole != controllerRole && createTokenRole != workerRole {
		cmd.SilenceUsage = true
		return fmt.Errorf("unsupported role %q, supported roles are %q and %q", createTokenRole, controllerRole, workerRole)
	}
	return nil
}
