package backup

import (
	"fmt"
	"github.com/k0sproject/k0s/internal/util"
	"github.com/sirupsen/logrus"
	"os"
	"path"
)

type configurationStep struct {
	path string
}

func newConfigurationStep(path string) *configurationStep {
	return &configurationStep{path: path}
}

func (c configurationStep) Name() string {
	return "k0s.yaml"
}

func (c configurationStep) Backup() (StepResult, error) {
	_, err := os.Stat(c.path)
	if os.IsNotExist(err) {
		logrus.Info("default k0s.yaml is used, do not back it up")
		return StepResult{}, nil
	}
	if err != nil {
		return StepResult{}, fmt.Errorf("can't backup `%s`: %v", c.path, err)
	}
	return StepResult{filesForBackup: []string{c.path}}, nil
}

func (c configurationStep) Restore(restoreFrom, restoreTo string) error {
	if !util.FileExists(restoreFrom) {
		logrus.Info("No k0s.yaml in the backup archive")
		return nil
	}
	logrus.Infof("Previously used k0s.yaml saved under the data directory `%s`", restoreTo)
	objectPathInArchive := path.Join(restoreFrom, "k0s.yaml")
	objectPathInRestored := path.Join(restoreTo, "k0s.yaml")
	return util.FileCopy(objectPathInArchive, objectPathInRestored)
}
