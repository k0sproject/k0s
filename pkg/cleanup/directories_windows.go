//go:build windows

// SPDX-FileCopyrightText: 2025 k0s authors
// SPDX-License-Identifier: Apache-2.0

package cleanup

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/sirupsen/logrus"
)

// Run removes the k0s data, kubelet root, and run directories.
func (d *directories) Run() error {
	paths := dedupePaths([]string{d.kubeletRootDir, d.dataDir, d.runDir})
	for _, path := range paths {
		if path == "" {
			continue
		}
		removeDirectory(path)
	}
	return nil
}

func removeDirectory(path string) {
	if out, err := exec.Command("powershell", "-NoProfile", "-Command",
		"Remove-Item", "-LiteralPath", path, "-Recurse", "-Force", "-ErrorAction", "SilentlyContinue",
	).CombinedOutput(); err != nil {
		logrus.WithError(err).Debugf("PowerShell Remove-Item for %s: %s", path, strings.TrimSpace(string(out)))
	}

	if _, err := os.Stat(path); os.IsNotExist(err) {
		return
	}

	logrus.Warnf("failed to delete %s, manual cleanup may be required", path)
}

func dedupePaths(paths []string) []string {
	seen := make(map[string]struct{}, len(paths))
	var result []string
	for _, p := range paths {
		cleaned := filepath.Clean(p)
		if cleaned == "." || cleaned == "" {
			continue
		}
		key := strings.ToLower(cleaned)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		result = append(result, cleaned)
	}
	return result
}
