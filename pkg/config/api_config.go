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
	"context"
	"fmt"
	"time"

	"github.com/imdario/mergo"
	"github.com/sirupsen/logrus"

	k0sv1beta1 "github.com/k0sproject/k0s/pkg/apis/k0s.k0sproject.io/clientset/typed/k0s.k0sproject.io/v1beta1"
	"github.com/k0sproject/k0s/pkg/apis/k0s.k0sproject.io/v1beta1"
	"github.com/k0sproject/k0s/pkg/constant"
)

// run a config-request from the API and wait until the API is up
func (rules *ClientConfigLoadingRules) getConfigFromAPI(client k0sv1beta1.K0sV1beta1Interface) (*v1beta1.ClusterConfig, error) {
	timeout := time.After(120 * time.Second)
	ticker := time.NewTicker(3 * time.Second)
	defer ticker.Stop()
	// Keep trying until we're timed out or got a result or got an error
	for {
		select {
		// Got a timeout! fail with a timeout error
		case <-timeout:
			return nil, fmt.Errorf("timed out waiting for API to return cluster-config")
		// Got a tick, we should check on doSomething()
		case <-ticker.C:
			logrus.Debug("fetching cluster-config from API...")
			cfg, err := rules.configRequest(client)
			if err != nil {
				continue
			}
			return cfg, nil
		}
	}
}

// when API config is enabled, but only node config is needed (for bootstrapping commands)
func (rules *ClientConfigLoadingRules) fetchNodeConfig() (*v1beta1.ClusterConfig, error) {
	cfg, err := rules.readRuntimeConfig()
	if err != nil {
		logrus.Errorf("failed to read config from file: %v", err)
		return nil, err
	}
	return cfg.GetBootstrappingConfig(), nil
}

// when API config is enabled, but only node config is needed (for bootstrapping commands)
func (rules *ClientConfigLoadingRules) mergeNodeAndClusterconfig(nodeConfig *v1beta1.ClusterConfig, apiConfig *v1beta1.ClusterConfig) (*v1beta1.ClusterConfig, error) {
	clusterConfig := &v1beta1.ClusterConfig{}

	// API config takes precedence over Node config. This is why we are merging it first
	err := mergo.Merge(clusterConfig, apiConfig)
	if err != nil {
		return nil, err
	}

	err = mergo.Merge(clusterConfig, nodeConfig.GetBootstrappingConfig(), mergo.WithOverride)
	if err != nil {
		return nil, err
	}

	return clusterConfig, nil
}

// fetch cluster-config from API
func (rules *ClientConfigLoadingRules) configRequest(client k0sv1beta1.K0sV1beta1Interface) (clusterConfig *v1beta1.ClusterConfig, err error) {
	clusterConfigs := client.ClusterConfigs(constant.ClusterConfigNamespace)
	ctxWithTimeout, cancelFunction := context.WithTimeout(context.Background(), time.Duration(10)*time.Second)
	defer cancelFunction()

	cfg, err := clusterConfigs.Get(ctxWithTimeout, "k0s", getOpts)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch cluster-config from API: %v", err)
	}
	return cfg, nil
}
