//go:build !windows
// +build !windows

package backup

import (
	"path"

	"sigs.k8s.io/yaml"

	"github.com/k0sproject/k0s/pkg/config"

	"github.com/sirupsen/logrus"

	"github.com/k0sproject/k0s/internal/pkg/file"
)

type configurationStep struct {
	restoredConfigPath string
}

func newConfigurationStep(path string, restoredConfigPath string) *configurationStep {
	return &configurationStep{
		restoredConfigPath: restoredConfigPath,
	}
}

func (c configurationStep) Name() string {
	return "k0s-config"
}

func (c configurationStep) Backup() (StepResult, error) {
	loadingrules := config.ClientConfigLoadingRules{}
	if loadingrules.IsDefaultConfig() {
		logrus.Info("default k0s config is used. not adding it to backup")
		return StepResult{}, nil
	}
	cfg, err := loadingrules.Load()
	if err != nil {
		logrus.Errorf("failed to fetch k0s config: %v", err)
	}
	cfgData, err := yaml.Marshal(cfg)
	if err != nil {
		logrus.Errorf("failed to marshal k0s config data: %v", err)
	}
	configPath, err := file.WriteTmpFile(string(cfgData), "k0s-config-backup")
	if err != nil {
		logrus.Errorf("failed to save config to file: %v", err)
	}
	return StepResult{filesForBackup: []string{configPath}}, nil
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
