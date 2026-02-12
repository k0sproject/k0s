// SPDX-FileCopyrightText: 2025 k0s authors
// SPDX-License-Identifier: Apache-2.0

package payload

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/k0sproject/k0s/cmd/internal"
	"github.com/k0sproject/k0s/pkg/assets"
	"github.com/k0sproject/k0s/pkg/constant"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

func NewExtractCmd() *cobra.Command {
	var dataDir string
	debugFlags := (&internal.DebugFlags{}).LongRunning()

	cmd := &cobra.Command{
		Use:   "extract",
		Short: "Extract embedded binaries",
		Args:  cobra.NoArgs,
		Long: `Extract all embedded binaries to the bin directory.

This command extracts all binaries embedded in the k0s executable to the
bin directory. By default, binaries are extracted to /var/lib/k0s/bin.

When a custom data-dir is specified, binaries are extracted to <data-dir>/bin.

This is useful for pre-staging binaries before starting k0s, especially in
airgapped environments or when the k0s binary is stored on a read-only
filesystem.

Examples:
  # Extract to default location (/var/lib/k0s/bin)
  k0s payload extract

  # Extract to custom location
  k0s payload extract --data-dir /custom/path
`,
		PersistentPreRun: debugFlags.Run,
		RunE: func(cmd *cobra.Command, args []string) error {
			return extractPayload(dataDir)
		},
	}

	cmd.Flags().StringVar(&dataDir, "data-dir", constant.DataDirDefault, "Data directory for k0s")
	debugFlags.AddToFlagSet(cmd.Flags())

	return cmd
}

func extractPayload(dataDir string) error {
	// Resolve absolute path
	absDataDir, err := filepath.Abs(dataDir)
	if err != nil {
		return fmt.Errorf("failed to resolve data directory path: %w", err)
	}

	binDir := filepath.Join(absDataDir, "bin")
	logrus.Infof("Extracting embedded binaries to %s", binDir)

	// Create bin directory if it doesn't exist
	if err := os.MkdirAll(binDir, 0755); err != nil {
		return fmt.Errorf("failed to create bin directory: %w", err)
	}

	// Get list of all embedded binaries
	binaries := assets.GetEmbeddedBinaries()
	if len(binaries) == 0 {
		return errors.New("no payload found")
	}

	// Extract each binary
	for _, binaryName := range binaries {
		_, err := assets.StageExecutable(binDir, binaryName)
		if err != nil {
			return fmt.Errorf("failed to stage %s: %w", binaryName, err)
		}

		logrus.Infof("Extracted %s", binaryName)
	}

	return nil
}
