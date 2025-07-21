// SPDX-FileCopyrightText: 2025 k0s authors
// SPDX-License-Identifier: Apache-2.0

package install

import (
	"github.com/spf13/cobra"
)

func addPlatformSpecificCommands(install *cobra.Command, installFlags *installFlags) {
	install.AddCommand(installControllerCmd(installFlags))
}
