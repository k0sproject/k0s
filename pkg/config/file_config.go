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
	"os"
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
	if rules.RuntimeConfigPath == "" {
		rules.RuntimeConfigPath = runtimeConfigPathDefault
	}
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

// ParseRuntimeConfig parses the `--config` flag and generates a config object
// it searches for the default config path. if it does not exist, and no other custom config-file is given, it will generate default config
func (rules *ClientConfigLoadingRules) ParseRuntimeConfig() (*v1beta1.ClusterConfig, error) {
	var cfg *v1beta1.ClusterConfig

	if rules.RuntimeConfigPath == "" {
		rules.RuntimeConfigPath = runtimeConfigPathDefault
	}

	// don't create the config file, if it already exists
	if file.Exists(rules.RuntimeConfigPath) {
		logrus.Infof("runtime config found: using %s", rules.RuntimeConfigPath)
	}

	switch CfgFile {
	case "-":
		// parse clusterConfig from stdin and write the contents to a temp directory
		return rules.readFromStdin()
	// no config-file is given, so either look for a config-file in the default location, or generate defaults
	case constant.K0sConfigPathDefault:
		// file doesn't exist, so we need to generate defaults
		if rules.IsDefaultConfig() {
			cfg = rules.generateDefaults()
		} else {
			return v1beta1.ConfigFromFile(constant.K0sConfigPathDefault, K0sVars.DataDir)
		}
	default:
		return v1beta1.ConfigFromFile(CfgFile, K0sVars.DataDir)
	}
	return cfg, nil
}

// InitRuntimeConfig generates the runtime /run/k0s/k0s.yaml
func (rules *ClientConfigLoadingRules) InitRuntimeConfig() error {
	cfg, err := rules.ParseRuntimeConfig()
	if err != nil {
		return err
	}

	yamlData, err := yaml.Marshal(&cfg)
	if err != nil {
		return err
	}

	return rules.writeConfig(yamlData)
}

// generate default config and return the file path
func (rules *ClientConfigLoadingRules) generateDefaults() (config *v1beta1.ClusterConfig) {
	logrus.Info("no config file given, generating default config")
	return v1beta1.DefaultClusterConfig(K0sVars.DataDir)
}

// generate config file from Stdin input
func (rules *ClientConfigLoadingRules) readFromStdin() (config *v1beta1.ClusterConfig, err error) {
	return v1beta1.ConfigFromStdin(K0sVars.DataDir)
}

func (rules *ClientConfigLoadingRules) writeConfig(yamlData []byte) error {
	mergedConfig, err := v1beta1.ConfigFromString(string(yamlData), K0sVars.DataDir)
	if err != nil {
		return fmt.Errorf("unable to parse config: %v", err)
	}
	data, err := yaml.Marshal(&mergedConfig)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %v", err)
	}
	err = os.WriteFile(rules.RuntimeConfigPath, data, 0755)
	if err != nil {
		return fmt.Errorf("failed to write runtime config to %s (%v): %v", K0sVars.RunDir, rules.RuntimeConfigPath, err)
	}
	return nil
}
