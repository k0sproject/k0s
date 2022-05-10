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
package config

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/k0sproject/k0s/internal/pkg/file"
	"github.com/k0sproject/k0s/pkg/apis/k0s.k0sproject.io/clientset/fake"
	k0sv1beta1 "github.com/k0sproject/k0s/pkg/apis/k0s.k0sproject.io/clientset/typed/k0s.k0sproject.io/v1beta1"
	"github.com/k0sproject/k0s/pkg/apis/k0s.k0sproject.io/v1beta1"
	"github.com/k0sproject/k0s/pkg/constant"
	"github.com/sirupsen/logrus"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var (
	configPathRuntimeTest = "/tmp/k0s.yaml"
	cOpts                 = v1.CreateOptions{TypeMeta: resourceType}
	fileYaml              = `
apiVersion: k0s.k0sproject.io/v1beta1
kind: ClusterConfig
spec:
  api:
    externalAddress: file_external_address
  network:
    serviceCIDR: 12.12.12.12/12
    podCIDR: 13.13.13.13/13
    kubeProxy:
      mode: ipvs	
`
	apiYaml = `
apiVersion: k0s.k0sproject.io/v1beta1
kind: ClusterConfig
spec:
  api:
    externalAddress: api_external_address
  network:
    serviceCIDR: api_cidr
`
)

// Test using config from a yaml file
func TestGetConfigFromFile(t *testing.T) {
	cfgFilePath := writeConfigFile(fileYaml)
	CfgFile = cfgFilePath
	defer os.Remove(configPathRuntimeTest)

	loadingRules := ClientConfigLoadingRules{RuntimeConfigPath: configPathRuntimeTest}
	err := loadingRules.InitRuntimeConfig(constant.GetConfig(""))
	if err != nil {
		t.Fatalf("failed to initialize k0s config: %s", err.Error())
	}

	cfg, err := loadingRules.Load()
	if err != nil {
		t.Fatalf("failed to load config: %s", err.Error())
	}
	if cfg == nil {
		t.Fatal("received an empty config! failing")
	}
	testCases := []struct {
		name     string
		got      string
		expected string
	}{
		{"API_external_address", cfg.Spec.API.ExternalAddress, "file_external_address"},
		{"Network_ServiceCIDR", cfg.Spec.Network.ServiceCIDR, "12.12.12.12/12"},
		{"Network_KubeProxy_Mode", cfg.Spec.Network.KubeProxy.Mode, "ipvs"},
	}

	for _, tc := range testCases {
		t.Run(fmt.Sprintf("%s eq %s", tc.name, tc.expected), func(t *testing.T) {
			if tc.got != tc.expected {
				t.Fatalf("expected to read '%s' for the %s test value. Got: %s", tc.expected, tc.name, tc.got)
			}
		})
	}
}

func TestExternalEtcdConfig(t *testing.T) {
	yamlData := `
spec:
  storage:
    type: etcd
    etcd:
      externalCluster:
        endpoints:
        - http://etcd0:2379
        etcdPrefix: k0s-tenant`
	cfgFilePath := writeConfigFile(yamlData)
	CfgFile = cfgFilePath
	defer os.Remove(configPathRuntimeTest)

	loadingRules := ClientConfigLoadingRules{RuntimeConfigPath: configPathRuntimeTest}
	err := loadingRules.InitRuntimeConfig(constant.GetConfig(""))
	if err != nil {
		t.Fatalf("failed to initialize k0s config: %s", err.Error())
	}

	cfg, err := loadingRules.Load()
	if err != nil {
		t.Fatalf("failed to load config: %s", err.Error())
	}
	if cfg == nil {
		t.Fatal("received an empty config! failing")
	}
	testCases := []struct {
		name     string
		got      string
		expected string
	}{
		{"Storage_Type", cfg.Spec.Storage.Type, "etcd"},
		{"External_Cluster_Endpoint", cfg.Spec.Storage.Etcd.ExternalCluster.Endpoints[0], "http://etcd0:2379"},
		{"External_Cluster_Prefix", cfg.Spec.Storage.Etcd.ExternalCluster.EtcdPrefix, "k0s-tenant"},
	}

	for _, tc := range testCases {
		t.Run(fmt.Sprintf("%s eq %s", tc.name, tc.expected), func(t *testing.T) {
			if tc.got != tc.expected {
				t.Fatalf("expected to read '%s' for the %s test value. Got: %s", tc.expected, tc.name, tc.got)
			}
		})
	}
}

// Test using config from a yaml file
func TestConfigFromDefaults(t *testing.T) {
	defer os.Remove(configPathRuntimeTest)

	CfgFile = ""
	loadingRules := ClientConfigLoadingRules{RuntimeConfigPath: configPathRuntimeTest}
	err := loadingRules.InitRuntimeConfig(constant.GetConfig(""))
	if err != nil {
		t.Fatalf("failed to initialize k0s config: %s", err.Error())
	}

	cfg, err := loadingRules.Load()
	if err != nil {
		t.Fatalf("failed to load config: %s", err.Error())
	}
	if cfg == nil {
		t.Fatal("received an empty config! failing")
	}
	testCases := []struct {
		name     string
		got      string
		expected string
	}{
		{"API_external_address", cfg.Spec.API.ExternalAddress, ""},
		{"Network_ServiceCIDR", cfg.Spec.Network.ServiceCIDR, "10.96.0.0/12"},
		{"Network_KubeProxy_Mode", cfg.Spec.Network.KubeProxy.Mode, "iptables"},
	}

	for _, tc := range testCases {
		t.Run(fmt.Sprintf("%s eq %s", tc.name, tc.expected), func(t *testing.T) {
			if tc.got != tc.expected {
				t.Fatalf("expected to read '%s' for the %s test value. Got: %s", tc.expected, tc.name, tc.got)
			}
		})
	}
}

// Test using node-config from a file when API config is enabled
func TestNodeConfigWithAPIConfig(t *testing.T) {
	cfgFilePath := writeConfigFile(fileYaml)
	CfgFile = cfgFilePath

	// if API config is enabled, Nodeconfig will be stripped of any cluster-wide-config settings
	controllerOpts.EnableDynamicConfig = true
	defer os.Remove(configPathRuntimeTest)

	loadingRules := ClientConfigLoadingRules{Nodeconfig: true, RuntimeConfigPath: configPathRuntimeTest}

	err := loadingRules.InitRuntimeConfig(constant.GetConfig(""))
	if err != nil {
		t.Fatalf("failed to initialize k0s config: %s", err.Error())
	}

	cfg, err := loadingRules.Load()
	if err != nil {
		t.Fatalf("failed to fetch Node Config: %s", err.Error())
	}
	testCases := []struct {
		name     string
		got      string
		expected string
	}{
		{"API_external_address", cfg.Spec.API.ExternalAddress, "file_external_address"},
		// PodCIDR is a cluster-wide setting. It shouldn't exist in Node config
		{"Network_PodCIDR", cfg.Spec.Network.PodCIDR, ""},
		{"Network_ServiceCIDR", cfg.Spec.Network.ServiceCIDR, "12.12.12.12/12"},
	}

	for _, tc := range testCases {
		t.Run(fmt.Sprintf("%s eq %s", tc.name, tc.expected), func(t *testing.T) {
			if tc.got != tc.expected {
				t.Fatalf("expected to read '%s' for the %s test value. Got: %s", tc.expected, tc.name, tc.got)
			}
		})
	}
}

func TestSingleNodeConfig(t *testing.T) {
	defer os.Remove(configPathRuntimeTest)

	loadingRules := ClientConfigLoadingRules{RuntimeConfigPath: configPathRuntimeTest, Nodeconfig: true}
	k0sVars := constant.GetConfig("")
	k0sVars.DefaultStorageType = "kine"
	CfgFile = ""

	err := loadingRules.InitRuntimeConfig(k0sVars)
	if err != nil {
		t.Fatalf("failed to initialize k0s config: %s", err.Error())
	}

	cfg, err := loadingRules.Load()
	if err != nil {
		t.Fatalf("failed to load config: %s", err.Error())
	}

	if cfg == nil {
		t.Fatal("received an empty config! failing")
	}

	testCases := []struct {
		name     string
		got      string
		expected string
	}{
		{"Storage_Type", cfg.Spec.Storage.Type, "kine"},
		{"Kine_DataSource", cfg.Spec.Storage.Kine.DataSource, "sqlite:///var/lib/k0s/db/state.db"},
	}

	for _, tc := range testCases {
		t.Run(fmt.Sprintf("%s eq %s", tc.name, tc.expected), func(t *testing.T) {
			if !strings.Contains(tc.got, tc.expected) {
				t.Fatalf("expected to read '%s' for the %s test value. Got: %s", tc.expected, tc.name, tc.got)
			}
		})
	}
}

func TestSingleNodeConfigWithEtcd(t *testing.T) {
	yamlData := `
spec:
  storage:
    type: etcd`

	cfgFilePath := writeConfigFile(yamlData)
	CfgFile = cfgFilePath
	defer os.Remove(configPathRuntimeTest)

	loadingRules := ClientConfigLoadingRules{RuntimeConfigPath: configPathRuntimeTest, Nodeconfig: true}
	k0sVars := constant.GetConfig("")
	k0sVars.DefaultStorageType = "kine"

	err := loadingRules.InitRuntimeConfig(k0sVars)
	if err != nil {
		t.Fatalf("failed to initialize k0s config: %s", err.Error())
	}

	cfg, err := loadingRules.Load()
	if err != nil {
		t.Fatalf("failed to load config: %s", err.Error())
	}
	if cfg == nil {
		t.Fatal("received an empty config! failing")
	}
	testCases := []struct {
		name     string
		got      string
		expected string
	}{{"Storage_Type", cfg.Spec.Storage.Type, "etcd"}} // config file storage type trumps k0sVars.DefaultStorageType

	for _, tc := range testCases {
		t.Run(fmt.Sprintf("%s eq %s", tc.name, tc.expected), func(t *testing.T) {
			if tc.got != tc.expected {
				t.Fatalf("expected to read '%s' for the %s test value. Got: %s", tc.expected, tc.name, tc.got)
			}
		})
	}
}

// when a component requests an API config,
// the merged node and cluster config should be returned
func TestAPIConfig(t *testing.T) {
	CfgFile = writeConfigFile(fileYaml)

	controllerOpts.EnableDynamicConfig = true
	// create the API config using a fake client
	client := fake.NewSimpleClientset()

	err := createFakeAPIConfig(client.K0sV1beta1())
	if err != nil {
		t.Fatalf("failed to create API config: %s", err.Error())
	}
	defer os.Remove(configPathRuntimeTest)

	loadingRules := ClientConfigLoadingRules{RuntimeConfigPath: configPathRuntimeTest, APIClient: client.K0sV1beta1()}
	err = loadingRules.InitRuntimeConfig(constant.GetConfig(""))
	if err != nil {
		t.Fatalf("failed to initialize k0s config: %s", err.Error())
	}

	cfg, err := loadingRules.Load()
	if err != nil {
		t.Fatalf("failed to fetch Node Config: %s", err.Error())
	}

	testCases := []struct {
		name     string
		got      string
		expected string
	}{
		{"API_external_address", cfg.Spec.API.ExternalAddress, "file_external_address"},
		{"Network_PodCIDR", cfg.Spec.Network.PodCIDR, "10.244.0.0/16"},
		{"Network_ServiceCIDR", cfg.Spec.Network.ServiceCIDR, "12.12.12.12/12"},
		{"Network_KubeProxy_Mode", cfg.Spec.Network.KubeProxy.Mode, "iptables"},
	}

	for _, tc := range testCases {
		t.Run(fmt.Sprintf("%s eq %s", tc.name, tc.expected), func(t *testing.T) {
			if tc.got != tc.expected {
				t.Fatalf("expected to read '%s' for the %s test value. Got: %s", tc.expected, tc.name, tc.got)
			}
		})
	}
}

func writeConfigFile(yamlData string) (filePath string) {
	cfgFilePath, err := file.WriteTmpFile(yamlData, "k0s-config")
	if err != nil {
		logrus.Fatalf("Error creating tempfile: %v", err)
	}
	return cfgFilePath
}

func createFakeAPIConfig(client k0sv1beta1.K0sV1beta1Interface) error {
	clusterConfigs := client.ClusterConfigs(constant.ClusterConfigNamespace)
	ctxWithTimeout, cancelFunction := context.WithTimeout(context.Background(), time.Duration(10)*time.Second)
	defer cancelFunction()

	config, err := v1beta1.ConfigFromString(apiYaml, v1beta1.DefaultStorageSpec())
	if err != nil {
		return fmt.Errorf("failed to parse config yaml: %s", err.Error())
	}

	_, err = clusterConfigs.Create(ctxWithTimeout, config.GetClusterWideConfig().StripDefaults(), cOpts)
	if err != nil {
		return fmt.Errorf("failed to create clusterConfig in the API: %s", err.Error())
	}
	return nil
}
