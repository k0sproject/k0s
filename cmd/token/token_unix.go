//go:build unix

// SPDX-FileCopyrightText: 2025 k0s authors
// SPDX-License-Identifier: Apache-2.0

package token

import (
	"github.com/spf13/cobra"
)

func addPlatformSpecificCommands(token *cobra.Command) {
	token.AddCommand(tokenCreateCmd())
}
