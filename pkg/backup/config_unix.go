//go:build unix

/*
Copyright 2021 k0s authors

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package backup

import (
	"fmt"
	"io"
	"os"
	"path"

	"github.com/k0sproject/k0s/internal/pkg/file"
	"github.com/sirupsen/logrus"
)

type configurationStep struct {
	cfgPath            string
	restoredConfigPath string
	out                io.Writer
}

func newConfigurationStep(cfgPath, restoredConfigPath string, out io.Writer) *configurationStep {
	return &configurationStep{
		cfgPath:            cfgPath,
		restoredConfigPath: restoredConfigPath,
		out:                out,
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
		logrus.Debugf("%s does not exist in the backup file", objectPathInArchive)
		return nil
	}

	logrus.Infof("Previously used k0s.yaml saved under the data directory `%s`", restoreTo)

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
