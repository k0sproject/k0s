//go:build unix

// SPDX-FileCopyrightText: 2021 k0s authors
// SPDX-License-Identifier: Apache-2.0

package backup

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/k0sproject/k0s/internal/pkg/file"
	"github.com/sirupsen/logrus"
)

// configFileName is the name the configuration has inside a backup archive,
// independent of the file name the cluster was started with.
const configFileName = "k0s.yaml"

type configurationStep struct {
	tmpDir             string
	cfgPath            string
	restoredConfigPath string
	out                io.Writer
}

func newConfigurationStep(tmpDir, cfgPath, restoredConfigPath string, out io.Writer) *configurationStep {
	return &configurationStep{
		tmpDir:             tmpDir,
		cfgPath:            cfgPath,
		restoredConfigPath: restoredConfigPath,
		out:                out,
	}
}

func (c configurationStep) Name() string {
	return "k0s-config"
}

func (c configurationStep) Backup() (StepResult, error) {
	if !file.Exists(c.cfgPath) {
		logrus.Warnf("configuration file %s does not exist, the backup archive won't contain one", c.cfgPath)
		return StepResult{}, nil
	}

	// Stage the configuration under the name that Restore looks for, so that
	// clusters started with a differently named file can be restored, too.
	staged := filepath.Join(c.tmpDir, configFileName)
	if err := file.Copy(c.cfgPath, staged); err != nil {
		return StepResult{}, fmt.Errorf("failed to stage configuration file %s: %w", c.cfgPath, err)
	}
	return StepResult{filesForBackup: []string{staged}}, nil
}

func (c configurationStep) Restore(restoreFrom, restoreTo string) error {
	objectPathInArchive := filepath.Join(restoreFrom, configFileName)

	if !file.Exists(objectPathInArchive) {
		logrus.Debugf("%s does not exist in the backup file", objectPathInArchive)
		return nil
	}

	if c.restoredConfigPath == "-" {
		f, err := os.Open(objectPathInArchive)
		if err != nil {
			return err
		}
		if f == nil {
			return fmt.Errorf("couldn't get a file handle for %s", c.restoredConfigPath)
		}
		defer f.Close()
		_, err = io.Copy(c.out, f)
		return err
	}

	logrus.Infof("restoring from `%s` to `%s`", objectPathInArchive, c.restoredConfigPath)
	return file.Copy(objectPathInArchive, c.restoredConfigPath)
}
