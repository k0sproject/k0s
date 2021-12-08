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
	"strings"
	"time"

	"github.com/imdario/mergo"

	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/clientcmd"

	cfgClient "github.com/k0sproject/k0s/pkg/apis/k0s.k0sproject.io/clientset"
	"github.com/k0sproject/k0s/pkg/apis/k0s.k0sproject.io/v1beta1"
	"github.com/k0sproject/k0s/pkg/constant"
	"github.com/sirupsen/logrus"
)

var (
	resourceType = v1.TypeMeta{APIVersion: "k0s.k0sproject.io/v1beta1", Kind: "clusterconfigs"}
	getOpts      = v1.GetOptions{TypeMeta: resourceType}
)

func getConfigFromAPI(kubeConfig string) (*v1beta1.ClusterConfig, error) {
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
			cfg, err := configRequest(kubeConfig)
			if err != nil {
				continue
			}
			return cfg, nil
		}
	}
}

// GetFullConfig returns the combined node & cluster config
func GetFullConfig(cfgPath string, k0sVars constant.CfgVars) (clusterConfig *v1beta1.ClusterConfig, err error) {
	if cfgPath == "" {
		// no config file exists, using defaults
		logrus.Warn("no config file given, using defaults")
	}
	cfg, err := ValidateYaml(cfgPath, k0sVars)
	if err != nil {
		return nil, err
	}

	apiConfig, err := getConfigFromAPI(k0sVars.AdminKubeConfigPath)
	if err != nil {
		return nil, err
	}
	if err := mergo.Merge(cfg, apiConfig, mergo.WithOverride); err != nil {
		return nil, err
	}
	return cfg, nil
}

// fetch cluster-config from API
func configRequest(kubeConfig string) (clusterConfig *v1beta1.ClusterConfig, err error) {
	config, err := clientcmd.BuildConfigFromFlags("", kubeConfig)
	if err != nil {
		return nil, fmt.Errorf("can't read kubeconfig: %v", err)
	}
	c, err := cfgClient.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("can't create kubernetes typed client for cluster config: %v", err)
	}

	clusterConfigs := c.K0sV1beta1().ClusterConfigs(constant.ClusterConfigNamespace)
	ctxWithTimeout, cancelFunction := context.WithTimeout(context.Background(), time.Duration(10)*time.Second)
	defer cancelFunction()

	cfg, err := clusterConfigs.Get(ctxWithTimeout, "k0s", getOpts)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch cluster-config from API: %v", err)
	}
	return cfg, nil
}

// GetYamlFromFile parses a yaml file into a ClusterConfig object
func GetYamlFromFile(cfgPath string, k0sVars constant.CfgVars) (clusterConfig *v1beta1.ClusterConfig, err error) {
	if cfgPath == "" {
		// no config file exists, using defaults
		logrus.Warn("no config file given, using defaults")
	}
	cfg, err := ValidateYaml(cfgPath, k0sVars)
	if err != nil {
		return nil, err
	}
	return cfg, nil
}

func ValidateYaml(cfgPath string, k0sVars constant.CfgVars) (clusterConfig *v1beta1.ClusterConfig, err error) {
	switch cfgPath {
	case "-":
		clusterConfig, err = v1beta1.ConfigFromStdin(k0sVars.DataDir)
	case "":
		clusterConfig = v1beta1.DefaultClusterConfig(k0sVars.DataDir)
	default:
		clusterConfig, err = v1beta1.ConfigFromFile(cfgPath, k0sVars.DataDir)
	}
	if err != nil {
		return nil, err
	}

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

// HACK: the current ClusterConfig struct holds both bootstrapping config & cluster-wide config
// this hack strips away the node-specific bootstrapping config so that we write a "clean" config to the CR
// This function accepts a standard ClusterConfig and returns the same config minus the node specific info:
// - APISpec
// - StorageSpec
// - Network.ServiceCIDR
// - Install
func ClusterConfigMinusNodeConfig(config *v1beta1.ClusterConfig) *v1beta1.ClusterConfig {
	clusterSpec := &v1beta1.ClusterSpec{
		ControllerManager: config.Spec.ControllerManager,
		Scheduler:         config.Spec.Scheduler,
		Network: &v1beta1.Network{
			Calico:     config.Spec.Network.Calico,
			DualStack:  config.Spec.Network.DualStack,
			KubeProxy:  config.Spec.Network.KubeProxy,
			KubeRouter: config.Spec.Network.KubeRouter,
			PodCIDR:    config.Spec.Network.PodCIDR,
			Provider:   config.Spec.Network.Provider,
		},
		PodSecurityPolicy: config.Spec.PodSecurityPolicy,
		WorkerProfiles:    config.Spec.WorkerProfiles,
		Telemetry:         config.Spec.Telemetry,
		Images:            config.Spec.Images,
		Extensions:        config.Spec.Extensions,
		Konnectivity:      config.Spec.Konnectivity,
		API: &v1beta1.APISpec{
			ExternalAddress: config.Spec.API.ExternalAddress,
			Address:         config.Spec.API.Address,
			Port:            config.Spec.API.Port,
		},
	}

	return &v1beta1.ClusterConfig{
		ObjectMeta: config.ObjectMeta,
		TypeMeta:   config.TypeMeta,
		DataDir:    config.DataDir,
		Spec:       clusterSpec,
		Status:     config.Status,
	}
}

// GetNodeConfig takes a config-file parameter and returns a ClusterConfig stripped of Cluster-Wide Settings
func GetNodeConfig(cfgPath string, k0sVars constant.CfgVars) (*v1beta1.ClusterConfig, error) {
	cfg, err := GetYamlFromFile(cfgPath, k0sVars)
	if err != nil {
		return nil, err
	}
	clusterSpec := &v1beta1.ClusterSpec{
		API: cfg.Spec.API,
		Storage: &v1beta1.StorageSpec{
			Type: cfg.Spec.Storage.Type,
			Etcd: &v1beta1.EtcdConfig{
				PeerAddress: cfg.Spec.Storage.Etcd.PeerAddress,
			},
			Kine: cfg.Spec.Storage.Kine,
		},
		Network: &v1beta1.Network{
			ServiceCIDR: cfg.Spec.Network.ServiceCIDR,
		},
		Install: cfg.Spec.Install,
	}
	nodeConfig := &v1beta1.ClusterConfig{
		ObjectMeta: cfg.ObjectMeta,
		TypeMeta:   cfg.TypeMeta,
		DataDir:    cfg.DataDir,
		Spec:       clusterSpec,
		Status:     cfg.Status,
	}
	return nodeConfig, nil
}
