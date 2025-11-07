// SPDX-FileCopyrightText: 2025 k0s authors
// SPDX-License-Identifier: Apache-2.0

package internal

import (
	"github.com/spf13/cobra"
)

// Runs the parent command's persistent pre-run. Cobra can do this either always
// for the whole command chain from root to leaf, or never. This function allows
// for more flexibility around this on a case by case basis.
//
// See: https://github.com/spf13/cobra/issues/216
func CallParentPersistentPreRun(cmd *cobra.Command, args []string) error {
	for p := cmd.Parent(); p != nil; p = p.Parent() {
		preRunE := p.PersistentPreRunE
		preRun := p.PersistentPreRun

		p.PersistentPreRunE = nil
		p.PersistentPreRun = nil

		defer func() {
			p.PersistentPreRunE = preRunE
			p.PersistentPreRun = preRun
		}()

		if preRunE != nil {
			return preRunE(cmd, args)
		}

		if preRun != nil {
			preRun(cmd, args)
			return nil
		}
	}

	return nil
}
