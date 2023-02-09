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

	"github.com/k0sproject/k0s/internal/pkg/file"
	"github.com/k0sproject/k0s/pkg/apis/k0s.k0sproject.io/v1beta1"
	"github.com/k0sproject/k0s/pkg/constant"
	"github.com/sirupsen/logrus"
	"sigs.k8s.io/yaml"
)

var (
	runtimeConfigPathDefault = "/run/k0s/k0s.yaml"
)

// InitRuntimeConfig generates the runtime /run/k0s/k0s.yaml
func (rules *ClientConfigLoadingRules) InitRuntimeConfig(k0sVars constant.CfgVars) error {
	rules.K0sVars = k0sVars
	cfg, err := rules.ParseRuntimeConfig()
	if err != nil {
		return err
	}

	yamlData, err := yaml.Marshal(cfg)
	if err != nil {
		return err
	}
	return rules.writeConfig(yamlData, cfg.Spec.Storage)
}

// readRuntimeConfig returns the configuration from the runtime configuration file
func (rules *ClientConfigLoadingRules) readRuntimeConfig() (clusterConfig *v1beta1.ClusterConfig, err error) {
	if rules.RuntimeConfigPath == "" {
		rules.RuntimeConfigPath = runtimeConfigPathDefault
	}

	config, err := rules.ParseRuntimeConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to parse config from %q: %w", rules.RuntimeConfigPath, err)
	}

	return config, err
}

// generic function that reads a config file, and returns a ClusterConfig object

// ParseRuntimeConfig parses the `--config` flag and generates a config object
// it searches for the default config path. if it does not exist, and no other custom config-file is given, it will generate default config
func (rules *ClientConfigLoadingRules) ParseRuntimeConfig() (*v1beta1.ClusterConfig, error) {
	var cfg *v1beta1.ClusterConfig

	var storage *v1beta1.StorageSpec
	if rules.K0sVars.DefaultStorageType == "kine" {
		storage = &v1beta1.StorageSpec{
			Type: v1beta1.KineStorageType,
			Kine: v1beta1.DefaultKineConfig(rules.K0sVars.DataDir),
		}
	}
	if rules.RuntimeConfigPath == "" {
		rules.RuntimeConfigPath = runtimeConfigPathDefault
	}

	// If runtime config already exists, use it as the source of truth
	if file.Exists(rules.RuntimeConfigPath) {
		logrus.Debugf("runtime config found: using %s", rules.RuntimeConfigPath)

		// read config from runtime config
		f, err := os.Open(rules.RuntimeConfigPath)
		if err != nil {
			return nil, err
		}
		defer f.Close()

		cfg, err = v1beta1.ConfigFromReader(f, storage)
		if err != nil {
			return nil, err
		}
		return cfg, nil
	}

	switch CfgFile {
	// stdin input
	case "-":
		return v1beta1.ConfigFromReader(os.Stdin, storage)
	case "":
		// if no config is set, look for config in the default location
		// if it does not exist there either, generate default config
		f, err := os.Open(constant.K0sConfigPathDefault)
		if err != nil {
			if os.IsNotExist(err) {
				return rules.generateDefaults(storage), nil
			}
		}
		defer f.Close()

		cfg, err = v1beta1.ConfigFromReader(f, storage)
		if err != nil {
			return nil, err
		}
	default:
		f, err := os.Open(CfgFile)
		if err != nil {
			return nil, err
		}
		defer f.Close()

		cfg, err = v1beta1.ConfigFromReader(f, storage)
		if err != nil {
			return nil, err
		}
	}
	if cfg.Spec.Storage.Type == v1beta1.KineStorageType && cfg.Spec.Storage.Kine == nil {
		logrus.Warn("storage type is kine but no config given, setting up defaults")
		cfg.Spec.Storage.Kine = v1beta1.DefaultKineConfig(rules.K0sVars.DataDir)
	}

	if cfg.Spec.Install == nil {
		cfg.Spec.Install = v1beta1.DefaultInstallSpec()
	}

	errors := cfg.Validate()
	if len(errors) > 0 {
		messages := make([]string, len(errors))
		for _, e := range errors {
			messages = append(messages, e.Error())
		}
		return nil, fmt.Errorf(strings.Join(messages, "\n"))
	}
	return cfg, nil
}

// generate default config and return the config object
func (rules *ClientConfigLoadingRules) generateDefaults(defaultStorage *v1beta1.StorageSpec) (config *v1beta1.ClusterConfig) {
	logrus.Debugf("no config file given, using defaults")
	return v1beta1.DefaultClusterConfig(defaultStorage)
}

func (rules *ClientConfigLoadingRules) writeConfig(yamlData []byte, storageSpec *v1beta1.StorageSpec) error {
	mergedConfig, err := v1beta1.ConfigFromString(string(yamlData), storageSpec)
	if err != nil {
		return fmt.Errorf("unable to parse config: %v", err)
	}
	data, err := yaml.Marshal(&mergedConfig)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %v", err)
	}

	err = file.WriteContentAtomically(rules.RuntimeConfigPath, data, 0755)
	if err != nil {
		return fmt.Errorf("failed to write runtime config to %s (%v): %v", rules.K0sVars.RunDir, rules.RuntimeConfigPath, err)
	}
	return nil
}
