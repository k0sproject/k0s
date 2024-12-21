/*
Copyright 2024 k0s authors

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
	"github.com/k0sproject/k0s/cmd/backup"
	"github.com/k0sproject/k0s/cmd/install"
	"github.com/k0sproject/k0s/cmd/reset"
	"github.com/k0sproject/k0s/cmd/restore"
	"github.com/k0sproject/k0s/cmd/start"
	"github.com/k0sproject/k0s/cmd/status"
	"github.com/k0sproject/k0s/cmd/stop"

	"github.com/spf13/cobra"
)

func addPlatformSpecificCommands(root *cobra.Command) {
	root.AddCommand(backup.NewBackupCmd())
	root.AddCommand(install.NewInstallCmd())
	root.AddCommand(reset.NewResetCmd())
	root.AddCommand(restore.NewRestoreCmd())
	root.AddCommand(start.NewStartCmd())
	root.AddCommand(status.NewStatusCmd())
	root.AddCommand(stop.NewStopCmd())
}
