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

	"github.com/k0sproject/k0s/internal/pkg/file"
	cfgClient "github.com/k0sproject/k0s/pkg/apis/k0s.k0sproject.io/clientset"
	k0sv1beta1 "github.com/k0sproject/k0s/pkg/apis/k0s.k0sproject.io/clientset/typed/k0s.k0sproject.io/v1beta1"
	"github.com/k0sproject/k0s/pkg/apis/k0s.k0sproject.io/v1beta1"
	"github.com/k0sproject/k0s/pkg/constant"
	"k8s.io/client-go/tools/clientcmd"
)

type K0sConfigGetter struct {
	k0sConfigGetter Getter
}

func (g *K0sConfigGetter) IsAPIConfig() bool {
	return false
}

func (g *K0sConfigGetter) IsDefaultConfig() bool {
	return false
}

func (g *K0sConfigGetter) BootstrapConfig() (*v1beta1.ClusterConfig, error) {
	return g.k0sConfigGetter()
}

func (g *K0sConfigGetter) Load() (*v1beta1.ClusterConfig, error) {
	return g.k0sConfigGetter()
}

type Getter func() (*v1beta1.ClusterConfig, error)

type ClientConfigLoadingRules struct {
	// APIClient is an optional field for passing a kubernetes API client, to fetch the API config
	// mostly used by tests, to pass a fake client
	APIClient k0sv1beta1.K0sV1beta1Interface

	// Nodeconfig is an optional field indicating if provided config-file is a node-config or a standard cluster-config file.
	Nodeconfig bool

	// RuntimeConfigPath is an optional field indicating the location of the runtime config file (default: /run/k0s/k0s.yaml)
	// this parameter is mainly used for testing purposes, to override the default location on local dev system
	RuntimeConfigPath string

	// K0sVars is needed for fetching the right config from the API
	K0sVars constant.CfgVars
}

func (rules *ClientConfigLoadingRules) BootstrapConfig() (*v1beta1.ClusterConfig, error) {
	return rules.fetchNodeConfig()
}

// ClusterConfig generates a client and queries the API for the cluster config
func (rules *ClientConfigLoadingRules) ClusterConfig() (*v1beta1.ClusterConfig, error) {
	if rules.APIClient == nil {
		// generate a kubernetes client from AdminKubeConfigPath
		config, err := clientcmd.BuildConfigFromFlags("", K0sVars.AdminKubeConfigPath)
		if err != nil {
			return nil, fmt.Errorf("can't read kubeconfig: %v", err)
		}
		client, err := cfgClient.NewForConfig(config)
		if err != nil {
			return nil, fmt.Errorf("can't create kubernetes typed client for cluster config: %v", err)
		}

		rules.APIClient = client.K0sV1beta1()
	}
	return rules.getConfigFromAPI(rules.APIClient)
}

func (rules *ClientConfigLoadingRules) IsAPIConfig() bool {
	return controllerOpts.EnableDynamicConfig
}

func (rules *ClientConfigLoadingRules) IsDefaultConfig() bool {
	// if no custom-value is provided as a config file, and no config-file exists in the default location
	// we assume we need to generate configuration defaults
	return CfgFile == constant.K0sConfigPathDefault && !file.Exists(constant.K0sConfigPathDefault)
}

func (rules *ClientConfigLoadingRules) Load() (*v1beta1.ClusterConfig, error) {
	if rules.Nodeconfig {
		return rules.fetchNodeConfig()
	}
	if !rules.IsAPIConfig() {
		return rules.readRuntimeConfig()
	}
	if rules.IsAPIConfig() {
		nodeConfig, err := rules.BootstrapConfig()
		if err != nil {
			return nil, err
		}
		apiConfig, err := rules.ClusterConfig()
		if err != nil {
			return nil, err
		}
		// get node config from the config-file and cluster-wide settings from the API and return a combined result
		return rules.mergeNodeAndClusterconfig(nodeConfig, apiConfig)
	}
	return nil, nil
}
