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

package start

import (
	"fmt"
	"os"

	"github.com/k0sproject/k0s/pkg/service"
	"github.com/spf13/cobra"
)

func NewStartCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "start",
		Short: "Start the k0s service configured on this host. Must be run as root (or with sudo)",
		RunE: func(cmd *cobra.Command, args []string) error {
			if os.Geteuid() != 0 {
				return fmt.Errorf("this command must be run as root")
			}
			svc, err := service.InstalledK0sService()
			if err != nil {
				return err
			}

			status, err := svc.Status()
			if err != nil {
				return err
			}
			if status == service.StatusRunning {
				return fmt.Errorf("already running")
			}

			if err := svc.Start(); err != nil {
				return fmt.Errorf("failed to start the service: %w", err)
			}

			return nil
		},
	}
}
