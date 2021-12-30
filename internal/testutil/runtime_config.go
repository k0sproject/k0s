/*
Copyright 2022 k0s authors

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
package testutil

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/sirupsen/logrus"
	"sigs.k8s.io/yaml"

	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/k0sproject/k0s/internal/pkg/file"
	"github.com/k0sproject/k0s/pkg/apis/k0s.k0sproject.io/clientset/fake"
	k0sv1beta1 "github.com/k0sproject/k0s/pkg/apis/k0s.k0sproject.io/clientset/typed/k0s.k0sproject.io/v1beta1"
	"github.com/k0sproject/k0s/pkg/apis/k0s.k0sproject.io/v1beta1"
	"github.com/k0sproject/k0s/pkg/config"
	"github.com/k0sproject/k0s/pkg/constant"
)

const RuntimeFakePath = "/tmp/k0s.yaml"

var resourceType = v1.TypeMeta{APIVersion: "k0s.k0sproject.io/v1beta1", Kind: "clusterconfigs"}

type ConfigGetter struct {
	NodeConfig bool
	YamlData   string

	k0sVars     constant.CfgVars
	cfgFilePath string
}

// NewConfigGetter sets the parameters required to fetch a fake config for testing
func NewConfigGetter(yamlData string, isNodeConfig bool, k0sVars constant.CfgVars) *ConfigGetter {
	return &ConfigGetter{
		YamlData:   yamlData,
		NodeConfig: isNodeConfig,
		k0sVars:    k0sVars,
	}
}

// FakeRuntimeConfig takes a yaml construct and returns a config object from a fake runtime config path
func (c *ConfigGetter) FakeConfigFromFile() (*v1beta1.ClusterConfig, error) {
	err := c.initRuntimeConfig()
	if err != nil {
		return nil, err
	}
	loadingRules := config.ClientConfigLoadingRules{
		RuntimeConfigPath: RuntimeFakePath,
		Nodeconfig:        c.NodeConfig,
		CfgFileOverride:   c.cfgFilePath,
		K0sVars:           c.k0sVars,
	}
	return loadingRules.Load()
}

func (c *ConfigGetter) FakeAPIConfig() (*v1beta1.ClusterConfig, error) {
	err := c.initRuntimeConfig()
	if err != nil {
		return nil, err
	}

	// create the API config using a fake client
	client := fake.NewSimpleClientset()

	err = c.createFakeAPIConfig(client.K0sV1beta1())
	if err != nil {
		return nil, fmt.Errorf("failed to get fake API client: %v", err)
	}

	loadingRules := config.ClientConfigLoadingRules{
		RuntimeConfigPath: RuntimeFakePath,
		Nodeconfig:        c.NodeConfig,
		CfgFileOverride:   c.cfgFilePath,
		APIClient:         client.K0sV1beta1(),
		K0sVars:           c.k0sVars,
	}

	return loadingRules.Load()
}

func (c *ConfigGetter) initRuntimeConfig() error {
	// write the yaml string into a temporary config file path
	cfgFilePath, err := file.WriteTmpFile(c.YamlData, "k0s-config")
	if err != nil {
		return fmt.Errorf("error creating tempfile: %v", err)
	}

	c.cfgFilePath = cfgFilePath

	logrus.Infof("using config path: %s", cfgFilePath)
	f, err := os.Open(c.cfgFilePath)
	if err != nil {
		return err
	}
	defer f.Close()

	mergedConfig, err := v1beta1.ConfigFromReader(f, c.getStorageSpec())
	if err != nil {
		return fmt.Errorf("unable to parse config from %s: %v", cfgFilePath, err)
	}
	data, err := yaml.Marshal(&mergedConfig)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %v", err)
	}
	err = os.WriteFile(RuntimeFakePath, data, 0755)
	if err != nil {
		return fmt.Errorf("failed to write runtime config to %s: %v", RuntimeFakePath, err)
	}
	return nil
}

func (c *ConfigGetter) createFakeAPIConfig(client k0sv1beta1.K0sV1beta1Interface) error {
	clusterConfigs := client.ClusterConfigs(constant.ClusterConfigNamespace)
	ctxWithTimeout, cancelFunction := context.WithTimeout(context.Background(), time.Duration(10)*time.Second)
	defer cancelFunction()

	cfg, err := v1beta1.ConfigFromString(c.YamlData, c.getStorageSpec())
	if err != nil {
		return fmt.Errorf("failed to parse config yaml: %s", err.Error())
	}

	_, err = clusterConfigs.Create(ctxWithTimeout, cfg.GetClusterWideConfig().StripDefaults(), v1.CreateOptions{TypeMeta: resourceType})
	if err != nil {
		return fmt.Errorf("failed to create clusterConfig in the API: %s", err.Error())
	}
	return nil
}

func (c *ConfigGetter) getStorageSpec() *v1beta1.StorageSpec {
	var storage *v1beta1.StorageSpec

	if c.k0sVars.DefaultStorageType == "kine" {
		storage = &v1beta1.StorageSpec{
			Type: v1beta1.KineStorageType,
			Kine: v1beta1.DefaultKineConfig(c.k0sVars.DataDir),
		}
	}
	return storage
}
