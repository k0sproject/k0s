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
	"os"
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
	cfg, err := GetConfigFromYAML(cfgPath, k0sVars)
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
	cfg, err := GetConfigFromYAML(cfgPath, k0sVars)
	if err != nil {
		return nil, err
	}
	return cfg, nil
}

// GetConfigFromYAML will attempt to read a config yaml, validate it and return a clusterConfig object
func GetConfigFromYAML(cfgPath string, k0sVars constant.CfgVars) (clusterConfig *v1beta1.ClusterConfig, err error) {
	var storage *v1beta1.StorageSpec
	var cfg *v1beta1.ClusterConfig

	CfgFile = cfgPath

	// first, let's set the default storage type
	if k0sVars.DefaultStorageType == "kine" {
		storage = &v1beta1.StorageSpec{
			Type: v1beta1.KineStorageType,
			Kine: v1beta1.DefaultKineConfig(k0sVars.DataDir),
		}
	}

	switch CfgFile {
	// read config file flag
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

	// stdin input
	case "-":
		cfg, err = v1beta1.ConfigFromReader(os.Stdin, storage)

	// config file not provided: try to read config from default location.
	// if not exists, generate default config
	case constant.K0sConfigPathDefault:
		f, err := os.Open(constant.K0sConfigPathDefault)
		if err != nil {
			if os.IsNotExist(err) {
				logrus.Debugf("could not find config in %s, using defaults", constant.K0sConfigPathDefault)
				cfg = v1beta1.DefaultClusterConfig(storage)
			} else {
				return nil, err
			}
		}
		if err == nil {
			logrus.Debugf("found config file in %s", constant.K0sConfigPathDefault)
			cfg, err = v1beta1.ConfigFromReader(f, storage)
			if err != nil {
				return nil, err
			}
			defer f.Close()
		}
	}

	if cfg.Spec.Storage.Type == v1beta1.KineStorageType && cfg.Spec.Storage.Kine == nil {
		logrus.Warn("storage type is kine but no config given, setting up defaults")
		cfg.Spec.Storage.Kine = v1beta1.DefaultKineConfig(k0sVars.DataDir)
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

// GetNodeConfig takes a config-file parameter and returns a ClusterConfig stripped of Cluster-Wide Settings
func GetNodeConfig(cfgPath string, k0sVars constant.CfgVars) (*v1beta1.ClusterConfig, error) {
	cfg, err := GetYamlFromFile(cfgPath, k0sVars)
	if err != nil {
		return nil, err
	}
	nodeConfig := cfg.GetBootstrappingConfig(cfg.Spec.Storage)
	var etcdConfig *v1beta1.EtcdConfig
	if cfg.Spec.Storage.Type == v1beta1.EtcdStorageType {
		etcdConfig = &v1beta1.EtcdConfig{
			ExternalCluster: cfg.Spec.Storage.Etcd.ExternalCluster,
			PeerAddress: 	 cfg.Spec.Storage.Etcd.PeerAddress,
		}
		nodeConfig.Spec.Storage.Etcd = etcdConfig
	}
	return nodeConfig, nil
}
