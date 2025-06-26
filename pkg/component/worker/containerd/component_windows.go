// SPDX-FileCopyrightText: 2025 k0s authors
// SPDX-License-Identifier: Apache-2.0

package containerd

import (
	"errors"
	"os"
	"path/filepath"

	"github.com/k0sproject/k0s/internal/pkg/file"
	"github.com/k0sproject/k0s/pkg/assets"
	"github.com/sirupsen/logrus"
)

const (
	defaultConfPath    = `C:\Program Files\containerd\config.toml`
	defaultImportsPath = `C:\etc\k0s\containerd.d\`
)

var executableNames = [...]string{
	"containerd.exe",
	"containerd-shim-runhcs-v1.exe",
}

func stageExecutable(dir, name string) error {
	err := assets.StageExecutable(dir, name)

	// Simply ignore the "running executable" problem for now. Whenever there's a
	// permission error and the target path is a file, log the error and continue.

	var pathErr *os.PathError
	if !errors.As(err, &pathErr) {
		return err
	}
	if pathErr.Path != filepath.Join(dir, name) {
		return err
	}
	if !errors.Is(pathErr.Err, os.ErrPermission) {
		return err
	}
	if !file.Exists(pathErr.Path) {
		return err
	}

	log := logrus.WithField("component", "containerd").WithError(err)
	log.Error("Failed to replace ", pathErr.Path, ", using existing executable")
	return nil
}
