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
	"errors"
	"fmt"
	"time"

	"github.com/k0sproject/k0s/pkg/component/status"
	"github.com/k0sproject/k0s/pkg/config"
	"github.com/k0sproject/k0s/pkg/token"

	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/util/retry"

	"github.com/spf13/cobra"
)

func tokenCreateCmd() *cobra.Command {
	var (
		createTokenRole string
		tokenExpiry     string
		waitCreate      bool
	)

	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create join token",
		Example: `k0s token create --role worker --expiry 100h //sets expiration time to 100 hours
k0s token create --role worker --expiry 10m  //sets expiration time to 10 minutes
`,
		PreRunE: func(cmd *cobra.Command, args []string) error {
			err := checkTokenRole(createTokenRole)
			if err != nil {
				cmd.SilenceUsage = true
			}
			return err
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			opts, err := config.GetCmdOpts(cmd)
			if err != nil {
				return err
			}

			expiry, err := time.ParseDuration(tokenExpiry)
			if err != nil {
				return err
			}

			var bootstrapToken string
			// we will retry every second for two minutes and then error
			err = retry.OnError(wait.Backoff{
				Steps:    120,
				Duration: 1 * time.Second,
				Factor:   1.0,
				Jitter:   0.1,
			}, func(err error) bool {
				return waitCreate
			}, func() error {
				statusInfo, err := status.GetStatusInfo(opts.K0sVars.StatusSocketPath)
				if err != nil {
					return fmt.Errorf("failed to get k0s status: %w", err)
				}
				if statusInfo == nil {
					return config.ErrK0sNotRunning
				}
				if err = ensureTokenCreationAcceptable(createTokenRole, statusInfo); err != nil {
					waitCreate = false
					cmd.SilenceUsage = true
					return err
				}

				nodeConfig, err := opts.K0sVars.NodeConfig()
				if err != nil {
					return err
				}

				bootstrapToken, err = token.CreateKubeletBootstrapToken(cmd.Context(), nodeConfig.Spec.API, opts.K0sVars, createTokenRole, expiry)
				return err
			})
			if err != nil {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), bootstrapToken)
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

func ensureTokenCreationAcceptable(createTokenRole string, statusInfo *status.K0sStatus) error {
	if statusInfo.SingleNode {
		return errors.New("refusing to create token: cannot join into a single node cluster")
	}
	if createTokenRole == token.RoleController && !statusInfo.ClusterConfig.Spec.Storage.IsJoinable() {
		return errors.New("refusing to create token: cannot join controller into current storage")
	}

	return nil
}
