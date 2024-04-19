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

package install

import (
	"errors"
	"fmt"
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
			case "env", "force":
				return
			case "data-dir", "token-file", "config":
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
