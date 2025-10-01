// SPDX-FileCopyrightText: 2025 k0s authors
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"github.com/k0sproject/k0s/cmd/backup"
	"github.com/k0sproject/k0s/cmd/controller"
	"github.com/k0sproject/k0s/cmd/keepalived"
	"github.com/k0sproject/k0s/cmd/reset"
	"github.com/k0sproject/k0s/cmd/restore"
	"github.com/k0sproject/k0s/cmd/status"

	"github.com/spf13/cobra"
)

func addPlatformSpecificCommands(root *cobra.Command) {
	root.AddCommand(backup.NewBackupCmd())
	root.AddCommand(controller.NewControllerCmd())
	root.AddCommand(keepalived.NewKeepalivedSetStateCmd()) // hidden
	root.AddCommand(reset.NewResetCmd())
	root.AddCommand(restore.NewRestoreCmd())
	root.AddCommand(status.NewStatusCmd())
}
