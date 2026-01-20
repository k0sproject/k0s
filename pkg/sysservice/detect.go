// SPDX-FileCopyrightText: 2026 k0s authors
// SPDX-License-Identifier: Apache-2.0

package sysservice

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

// Detect returns the best guess service backend kind: "systemd", "openrc", or "windows".
// If root != "", it will check marker paths under that root first (useful for tests).
func Detect(root string) (string, error) {
	if runtime.GOOS == "windows" {
		return "windows", nil
	}

	// Prefer filesystem markers (most reliable), then fall back to PATH checks.
	if exists(root, "/run/systemd/system") {
		return "systemd", nil
	}
	if exists(root, "/run/openrc") {
		return "openrc", nil
	}

	// PATH fallbacks (for real installs where markers might not be visible)
	if _, err := exec.LookPath("systemctl"); err == nil {
		return "systemd", nil
	}
	if _, err := exec.LookPath("openrc-init"); err == nil {
		return "openrc", nil
	}
	if _, err := exec.LookPath("rc-service"); err == nil {
		return "openrc", nil
	}

	return "", errors.New("unable to detect init/service manager (systemd/openrc)")
}

func exists(root, absPath string) bool {
	// absPath must be absolute (starts with "/") for this helper.
	p := absPath
	if root != "" {
		// Join root with absPath without losing absPath semantics
		p = filepath.Join(root, absPath[1:]) // strip leading slash
	}
	_, err := os.Stat(p)
	return err == nil
}
