// SPDX-FileCopyrightText: 2021 k0s authors
// SPDX-License-Identifier: Apache-2.0

package install

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

func cmdFlagsToArgs(cmd *cobra.Command) ([]string, error) {
	var flagsAndVals []string
	var errs []error
	// Use visitor to collect all flags and vals into slice
	cmd.Flags().Visit(func(f *pflag.Flag) {
		val := f.Value.String()
		switch f.Value.Type() {
		case "stringSlice", "stringToString":
			flagsAndVals = append(flagsAndVals, fmt.Sprintf(`--%s=%s`, f.Name, strings.Trim(val, "[]")))
		default:
			switch f.Name {
			case "env", "force", "token-env":
				return
			case "data-dir", "kubelet-root-dir", "token-file", "config":
				if absVal, err := filepath.Abs(val); err != nil {
					err = fmt.Errorf("failed to convert --%s=%s to an absolute path: %w", f.Name, val, err)
					errs = append(errs, err)
				} else {
					val = absVal
				}
			}
			flagsAndVals = append(flagsAndVals, fmt.Sprintf("--%s=%s", f.Name, val))
		}
	})

	if err := errors.Join(errs...); err != nil {
		return nil, err
	}

	return flagsAndVals, nil
}

// handleTokenEnv converts --token-env to a token file and returns its path.
func handleTokenEnv(cmd *cobra.Command, dataDir string) (string, error) {
	tokenEnvFlag := cmd.Flags().Lookup("token-env")
	if tokenEnvFlag == nil || !tokenEnvFlag.Changed {
		return "", nil
	}

	envVarName := tokenEnvFlag.Value.String()
	tokenValue := os.Getenv(envVarName)
	if tokenValue == "" {
		return "", fmt.Errorf("environment variable %q is not set or is empty", envVarName)
	}

	tokenFilePath := filepath.Join(dataDir, ".token")
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create data directory: %w", err)
	}

	if err := os.WriteFile(tokenFilePath, []byte(tokenValue), 0600); err != nil {
		return "", fmt.Errorf("failed to write token file: %w", err)
	}

	return tokenFilePath, nil
}
