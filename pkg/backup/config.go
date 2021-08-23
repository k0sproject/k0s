// +build !windows

package backup

import (
	"fmt"
	"os"
	"path"

	"github.com/sirupsen/logrus"

	"github.com/k0sproject/k0s/internal/util"
)

type configurationStep struct {
	path               string
	restoredConfigPath string
	logger             *logrus.Logger
}

func newConfigurationStep(path string, restoredConfigPath string) *configurationStep {
	return &configurationStep{
		path:               path,
		restoredConfigPath: restoredConfigPath,
		logger:             util.CLILogger(),
	}
}

func (c configurationStep) Name() string {
	return c.path
}

func (c configurationStep) Backup() (StepResult, error) {
	_, err := os.Stat(c.path)
	if os.IsNotExist(err) {
		c.logger.Info("default k0s.yaml is used, do not back it up")
		return StepResult{}, nil
	}
	if err != nil {
		return StepResult{}, fmt.Errorf("can't backup `%s`: %v", c.path, err)
	}
	return StepResult{filesForBackup: []string{c.path}}, nil
}

func (c configurationStep) Restore(restoreFrom, restoreTo string) error {
	objectPathInArchive := path.Join(restoreFrom, "k0s.yaml")

	if !util.FileExists(objectPathInArchive) {
		c.logger.Infof("%s does not exist in the backup file", objectPathInArchive)
		return nil
	}
	c.logger.Infof("Previously used k0s.yaml saved under the data directory `%s`", restoreTo)

	c.logger.Infof("restoring from `%s` to `%s`", objectPathInArchive, c.restoredConfigPath)
	return util.FileCopy(objectPathInArchive, c.restoredConfigPath)
}
