//go:build unix

// SPDX-FileCopyrightText: 2021 k0s authors
// SPDX-License-Identifier: Apache-2.0

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
		Args: cobra.NoArgs,
		PreRunE: func(cmd *cobra.Command, _ []string) error {
			return checkTokenRole(createTokenRole)
		},
		RunE: func(cmd *cobra.Command, _ []string) error {
			opts, err := config.GetCmdOpts(cmd)
			if err != nil {
				return err
			}

			expiry, err := time.ParseDuration(tokenExpiry)
			if err != nil {
				return err
			}

			nodeConfig, err := opts.K0sVars.NodeConfig()
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

	flags := cmd.Flags()
	flags.AddFlagSet(config.GetPersistentFlagSet())
	flags.StringVar(&tokenExpiry, "expiry", "0s", "Expiration time of the token. Format 1.5h, 2h45m or 300ms.")
	flags.StringVar(&createTokenRole, "role", "worker", "Either worker or controller")
	flags.BoolVar(&waitCreate, "wait", false, "wait forever (default false)")

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
