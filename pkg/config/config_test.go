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
    serviceCIDR: file_service_cidr
    podCIDR: file_pod_cidr
    kubeProxy:
      mode: testFileConfig	
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

	loadingRules := ClientConfigLoadingRules{RuntimeConfigPath: "/tmp/k0s.yaml"}
	err := loadingRules.InitRuntimeConfig()
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
		{"Network_ServiceCIDR", cfg.Spec.Network.ServiceCIDR, "file_service_cidr"},
		{"Network_KubeProxy_Mode", cfg.Spec.Network.KubeProxy.Mode, "testFileConfig"},
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
	CfgFile = constant.K0sConfigPathDefault // this path doesn't exist, so default values should be generated
	loadingRules := ClientConfigLoadingRules{RuntimeConfigPath: "/tmp/k0s.yaml"}
	err := loadingRules.InitRuntimeConfig()
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
	loadingRules := ClientConfigLoadingRules{Nodeconfig: true, RuntimeConfigPath: "/tmp/k0s.yaml"}

	err := loadingRules.InitRuntimeConfig()
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
		{"API_external_address", cfg.Spec.API.ExternalAddress, ""},
		// PodCIDR is a cluster-wide setting. It shouldn't exist in Node config
		{"Network_PodCIDR", cfg.Spec.Network.PodCIDR, ""},
		{"Network_ServiceCIDR", cfg.Spec.Network.ServiceCIDR, "file_service_cidr"},
	}

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
	cfgFilePath := writeConfigFile(fileYaml)
	CfgFile = cfgFilePath

	controllerOpts.EnableDynamicConfig = true

	// create the API config using a fake client
	client := fake.NewSimpleClientset()

	err := createFakeAPIConfig(client.K0sV1beta1())
	if err != nil {
		t.Fatalf("failed to create API config: %s", err.Error())
	}

	loadingRules := ClientConfigLoadingRules{RuntimeConfigPath: "/tmp/k0s.yaml", APIClient: client.K0sV1beta1()}

	err = loadingRules.InitRuntimeConfig()
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
		// API Config should take precedence over Node config file
		{"API_external_address", cfg.Spec.API.ExternalAddress, "api_external_address"},
		{"Network_PodCIDR", cfg.Spec.Network.PodCIDR, "10.244.0.0/16"},
		{"Network_ServiceCIDR", cfg.Spec.Network.ServiceCIDR, "file_service_cidr"},
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

	config, err := v1beta1.ConfigFromString(apiYaml, "")
	if err != nil {
		return fmt.Errorf("failed to parse config yaml: %s", err.Error())
	}

	_, err = clusterConfigs.Create(ctxWithTimeout, config.GetClusterWideConfig().StripDefaults(), cOpts)
	if err != nil {
		return fmt.Errorf("failed to create clusterConfig in the API: %s", err.Error())
	}
	return nil
}
