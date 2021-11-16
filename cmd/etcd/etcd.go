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
package etcd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/k0sproject/k0s/pkg/apis/k0s.k0sproject.io/v1beta1"
	"github.com/k0sproject/k0s/pkg/config"
)

type CmdOpts config.CLIOptions

func NewEtcdCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "etcd",
		Short: "Manage etcd cluster",
		PreRunE: func(cmd *cobra.Command, args []string) error {
			c := CmdOpts(config.GetCmdOpts())
			// get k0s config
			loadingRules := config.ClientConfigLoadingRules{Nodeconfig: true}
			cfg, err := loadingRules.Load()
			if err != nil {
				return err
			}
			c.NodeConfig = cfg
			if c.NodeConfig.Spec.Storage.Type != v1beta1.EtcdStorageType {
				return fmt.Errorf("wrong storage type: %s", c.ClusterConfig.Spec.Storage.Type)
			}
			return nil
		},
	}
	cmd.SilenceUsage = true
	cmd.AddCommand(etcdLeaveCmd())
	cmd.AddCommand(etcdListCmd())
	cmd.PersistentFlags().AddFlagSet(config.GetPersistentFlagSet())
	return cmd
}
