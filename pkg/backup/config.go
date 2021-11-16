//go:build !windows
// +build !windows

package backup

import (
	"path"

	"github.com/sirupsen/logrus"

	"github.com/k0sproject/k0s/internal/pkg/file"
)

type configurationStep struct {
	cfgPath            string
	restoredConfigPath string
}

func newConfigurationStep(cfgPath string, restoredConfigPath string) *configurationStep {
	return &configurationStep{
		cfgPath:            cfgPath,
		restoredConfigPath: restoredConfigPath,
	}
}

func (c configurationStep) Name() string {
	return "k0s-config"
}

func (c configurationStep) Backup() (StepResult, error) {
	return StepResult{filesForBackup: []string{c.cfgPath}}, nil
}

func (c configurationStep) Restore(restoreFrom, restoreTo string) error {
	objectPathInArchive := path.Join(restoreFrom, "k0s.yaml")

	if !file.Exists(objectPathInArchive) {
		logrus.Infof("%s does not exist in the backup file", objectPathInArchive)
		return nil
	}
	logrus.Infof("Previously used k0s.yaml saved under the data directory `%s`", restoreTo)

	logrus.Infof("restoring from `%s` to `%s`", objectPathInArchive, c.restoredConfigPath)
	return file.Copy(objectPathInArchive, c.restoredConfigPath)
}
