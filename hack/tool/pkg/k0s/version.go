// SPDX-FileCopyrightText: 2022 k0s authors
// SPDX-License-Identifier: Apache-2.0

package k0s

import (
	"os/exec"
	"strings"
)

// Version returns the version of the k0s binary at the provided path.
func Version(k0sBinaryPath string) (string, error) {
	out, err := exec.Command(k0sBinaryPath, "version").Output()
	if err != nil {
		return "", err
	}

	return strings.TrimSpace(string(out)), nil
}
