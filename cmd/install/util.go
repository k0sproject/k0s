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

func resolveEnvVars(in []string) ([]string, error) {
	out := make([]string, len(in))
	for i, evar := range in {
		if strings.Contains(evar, "\x00") {
			return nil, errors.New("NUL byte in environment variable")
		}
		if name, _, hasValue := strings.Cut(evar, "="); hasValue {
			out[i] = evar
		} else {
			out[i] = name + "=" + os.Getenv(name)
		}
	}

	return out, nil
}

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
			case "env", "force", "start":
				return
			case "data-dir", "kubelet-root-dir", "containerd-root-dir", "token-file", "config":
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
