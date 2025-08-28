// SPDX-FileCopyrightText: 2025 k0s authors
// SPDX-License-Identifier: Apache-2.0

package containerd

import (
	"errors"
	"os"

	"github.com/k0sproject/k0s/internal/pkg/file"
	"github.com/k0sproject/k0s/pkg/assets"
	"github.com/sirupsen/logrus"
)

const (
	defaultConfPath    = `C:\Program Files\containerd\config.toml`
	defaultImportsPath = `C:\etc\k0s\containerd.d\`
)

var additionalExecutableNames = [...]string{
	"containerd-shim-runhcs-v1",
}

func stageExecutable(dir, name string) (string, error) {
	path, err := assets.StageExecutable(dir, name)

	// Simply ignore the "running executable" problem for now. Whenever there's a
	// permission error and the target path is a file, log the error and continue.

	// The assets.StageExecutable function is returning the path, even under
	// error conditions. This is kind of a hack to support the use case at hand.
	// Be defensive and check if that's actually the case, and don't swallow the
	// error if there's no path at all.
	if path != "" && isRunningExecutable(err, path) {
		log := logrus.WithField("component", "containerd").WithError(err)
		log.Error("Failed to replace ", path, ", using existing executable")
		return path, nil
	}

	return path, err
}

func isRunningExecutable(err error, path string) bool {
	var pathErr *os.PathError
	if !errors.As(err, &pathErr) {
		return false
	}
	if pathErr.Path != path {
		return false
	}
	if !errors.Is(pathErr.Err, os.ErrPermission) {
		return false
	}
	if !file.Exists(path) {
		return false
	}

	return true
}
