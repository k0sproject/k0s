/*
Copyright 2020 k0s authors

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
	"context"
	"encoding/json"
	"fmt"

	"github.com/k0sproject/k0s/pkg/config"
	"github.com/k0sproject/k0s/pkg/etcd"

	"github.com/spf13/cobra"
)

func etcdListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "member-list",
		Short: "Returns etcd cluster members list",
		RunE: func(cmd *cobra.Command, args []string) error {
			c := config.GetCmdOpts()
			ctx := context.Background()
			etcdClient, err := etcd.NewClient(c.K0sVars.CertRootDir, c.K0sVars.EtcdCertDir, c.NodeConfig.Spec.Storage.Etcd)
			if err != nil {
				return fmt.Errorf("can't list etcd cluster members: %v", err)
			}
			members, err := etcdClient.ListMembers(ctx)
			if err != nil {
				return fmt.Errorf("can't list etcd cluster members: %v", err)
			}
			return json.NewEncoder(cmd.OutOrStdout()).Encode(map[string]interface{}{"members": members})
		},
	}
	cmd.PersistentFlags().AddFlagSet(config.GetPersistentFlagSet())
	return cmd
}
