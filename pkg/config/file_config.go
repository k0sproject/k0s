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
package config

import (
	"fmt"
	"strings"

	"github.com/sirupsen/logrus"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/yaml"

	"github.com/k0sproject/k0s/internal/pkg/file"
	"github.com/k0sproject/k0s/pkg/apis/k0s.k0sproject.io/v1beta1"
	"github.com/k0sproject/k0s/pkg/constant"
)

var (
	resourceType             = v1.TypeMeta{APIVersion: "k0s.k0sproject.io/v1beta1", Kind: "clusterconfigs"}
	getOpts                  = v1.GetOptions{TypeMeta: resourceType}
	runtimeConfigPathDefault = "/run/k0s/k0s.yaml"
)

// readRuntimeConfig returns the configuration from the runtime configuration file
func (rules *ClientConfigLoadingRules) readRuntimeConfig() (clusterConfig *v1beta1.ClusterConfig, err error) {
	clusterConfig, err = v1beta1.ConfigFromFile(rules.RuntimeConfigPath, "")
	if err != nil {
		return nil, err
	}
	return clusterConfig, nil
}

// validate accepts a ClusterConfig object and validates config fields
func (rules *ClientConfigLoadingRules) Validate(clusterConfig *v1beta1.ClusterConfig, k0sVars constant.CfgVars) (*v1beta1.ClusterConfig, error) {
	if clusterConfig.Spec.Storage.Type == v1beta1.KineStorageType && clusterConfig.Spec.Storage.Kine == nil {
		logrus.Warn("storage type is kine but no config given, setting up defaults")
		clusterConfig.Spec.Storage.Kine = v1beta1.DefaultKineConfig(k0sVars.DataDir)
	}
	if clusterConfig.Spec.Install == nil {
		clusterConfig.Spec.Install = v1beta1.DefaultInstallSpec()
	}

	errors := clusterConfig.Validate()
	if len(errors) > 0 {
		messages := make([]string, len(errors))
		for _, e := range errors {
			messages = append(messages, e.Error())
		}
		return nil, fmt.Errorf(strings.Join(messages, "\n"))
	}
	return clusterConfig, nil
}

// InitRuntimeConfig generates the runtime /run/k0s/k0s.yaml
// it searches for the default config path. if it does not exist, and no other custom config-file is given, it will generate default config
// and place it in a tmp directory.
func (rules *ClientConfigLoadingRules) InitRuntimeConfig() error {
	var configPath string
	var err error

	if rules.RuntimeConfigPath == "" {
		rules.RuntimeConfigPath = runtimeConfigPathDefault
	}

	switch CfgFile {
	case "-":
		// parse clusterConfig from stdin and write the contents to a temp directory
		configPath, err = rules.readFromStdin()
		if err != nil {
			return err
		}

	// no config-file is given, so either look for a config-file in the default location, or generate defaults
	case constant.K0sConfigPathDefault:
		// file doesn't exist, so we need to generate defaults
		if rules.IsDefaultConfig() {
			configPath, err = rules.generateDefaults()
			if err != nil {
				return err
			}
		} else {
			configPath = constant.K0sConfigPathDefault
		}
	// other value is provided - in which case, simply create a symlink
	default:
		configPath = CfgFile
	}
	return rules.copyConfig(configPath)
}

// generate default config and return the file path
func (rules *ClientConfigLoadingRules) generateDefaults() (filePath string, err error) {
	logrus.Info("no config file given, generating default config")
	// generate default configuration and dump into a tmp file
	clusterConfig := v1beta1.DefaultClusterConfig(K0sVars.DataDir)
	// write to tmp dir
	yamlData, err := yaml.Marshal(clusterConfig)
	if err != nil {
		return "", fmt.Errorf("failed to marshal cluster-config: %v", err)
	}
	return file.WriteTmpFile(string(yamlData), "k0s-config")
}

// generate config file from Stdin input
func (rules *ClientConfigLoadingRules) readFromStdin() (filePath string, err error) {
	clusterConfig, err := v1beta1.ConfigFromStdin(K0sVars.DataDir)
	if err != nil {
		return "", fmt.Errorf("failed to read cluster-config from stdin: %v", err)
	}
	yamlData, err := yaml.Marshal(clusterConfig)
	if err != nil {
		return "", fmt.Errorf("failed to marshal cluster-config: %v", err)
	}
	return file.WriteTmpFile(string(yamlData), "k0s-config")
}

func (rules *ClientConfigLoadingRules) copyConfig(path string) error {
	// link the config file to /run/k0s/k0s.yaml
	logrus.Infof("using config path: %s", path)
	err := file.Copy(path, rules.RuntimeConfigPath)
	if err != nil {
		return fmt.Errorf("failed to copy k0s config to %s (%v): %v", K0sVars.RunDir, rules.RuntimeConfigPath, err)
	}
	return nil
}
