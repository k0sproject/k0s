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
package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/sirupsen/logrus"

	config "github.com/k0sproject/k0s/pkg/apis/v1beta1"
)

// ConfigFromYaml returns given k0s config or default config
func ConfigFromYaml(cfgPath string) (clusterConfig *config.ClusterConfig, err error) {
	if cfgPath == "" {
		logrus.Info("no config file given, using defaults")
		clusterConfig = config.DefaultClusterConfig(k0sVars)
	} else if isInputFromPipe() {
		clusterConfig, err = config.FromYamlPipe(os.Stdin, k0sVars)
	} else {
		clusterConfig, err = config.FromYamlFile(cfgPath, k0sVars)
	}

	if err != nil {
		return nil, err
	}
	// validate
	errors := clusterConfig.Validate()
	if len(errors) > 0 {
		messages := make([]string, len(errors))
		for _, e := range errors {
			messages = append(messages, e.Error())
		}
		return nil, fmt.Errorf(strings.Join(messages, "\n"))
	}
	if clusterConfig.Spec.Storage.Type == config.KineStorageType && clusterConfig.Spec.Storage.Kine == nil {
		clusterConfig.Spec.Storage.Kine = config.DefaultKineConfig(k0sVars.DataDir)
	}
	if clusterConfig.Spec.Install == nil {
		clusterConfig.Spec.Install = config.DefaultInstallSpec()
	}
	return clusterConfig, nil
}

func isInputFromPipe() bool {
	fi, _ := os.Stdin.Stat()
	return fi.Mode()&os.ModeCharDevice == 0
}
