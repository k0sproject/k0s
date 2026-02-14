//go:build !linux

// SPDX-FileCopyrightText: 2025 k0s authors
// SPDX-License-Identifier: Apache-2.0

package cmd

import "github.com/spf13/cobra"

func addPlatformSpecificCommands(root *cobra.Command) { /* no-op */ }
