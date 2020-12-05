/*
Copyright 2020 Mirantis, Inc.

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
package server

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/ghodss/yaml"
	config "github.com/k0sproject/k0s/pkg/apis/v1beta1"
	"github.com/k0sproject/k0s/pkg/constant"

	"github.com/stretchr/testify/assert"
)

var k0sVars = constant.GetConfig("")

func Test_KubeletConfig(t *testing.T) {
	dnsAddr := "dns.local"
	volumePluginDir := k0sVars.KubeletVolumePluginDir
	clientCAFile := filepath.Join(k0sVars.CertRootDir, "ca.crt")

	t.Run("default_profile_only", func(t *testing.T) {
		k, err := NewKubeletConfig(config.DefaultClusterConfig().Spec, k0sVars)
		assert.NoError(t, err)
		buf, err := k.run(dnsAddr)
		assert.NoError(t, err)
		manifestYamls := strings.Split(strings.TrimSuffix(buf.String(), "---"), "---")[1:]
		t.Run("output_must_have_3_manifests", func(t *testing.T) {
			assert.Len(t, manifestYamls, 3, "Must have exactly 3 generated manifests per profile")
			assertConfigMap(t, manifestYamls[0], "kubelet-config-default-1.19")
			assertRole(t, manifestYamls[1], []string{formatProfileName("default")})
			assertRoleBinding(t, manifestYamls[2])
		})
	})
	t.Run("with_user_provided_profiles", func(t *testing.T) {
		k := defaultConfigWithUserProvidedProfiles(t)
		buf, err := k.run(dnsAddr)
		assert.NoError(t, err)
		manifestYamls := strings.Split(strings.TrimSuffix(buf.String(), "---"), "---")[1:]
		expectedManifestsCount := 3 + 2 // 3 manifests per profile, 2 user profiles and 1 default
		assert.Len(t, manifestYamls, expectedManifestsCount, "Must have exactly 3 generated manifests per profile")

		t.Run("final_output_must_have_manifests_for_profiles", func(t *testing.T) {
			// check that each profile has config map, role and role binding
			resourceNamesForRole := []string{}
			for idx, profileName := range []string{"default", "profile_XXX", "profile_YYY"} {
				fullName := "kubelet-config-" + profileName + "-1.19"
				resourceNamesForRole = append(resourceNamesForRole, formatProfileName(profileName))
				assertConfigMap(t, manifestYamls[idx], fullName)
			}
			assertRole(t, manifestYamls[len(resourceNamesForRole)], resourceNamesForRole)
		})
		t.Run("user_profile_X_must_be_merged_with_default_profile", func(t *testing.T) {

			profileXXX := struct {
				Data map[string]string `yaml:"data"`
			}{}

			profileYYY := struct {
				Data map[string]string `yaml:"data"`
			}{}

			assert.NoError(t, yaml.Unmarshal([]byte(manifestYamls[1]), &profileXXX))
			assert.NoError(t, yaml.Unmarshal([]byte(manifestYamls[2]), &profileYYY))

			// manually apple the same changes to default config and check that there is no diff
			defaultProfileKubeletConfig := getDefaultProfile(dnsAddr, clientCAFile, volumePluginDir)
			defaultProfileKubeletConfig["authentication"].(map[string]interface{})["anonymous"].(map[string]interface{})["enabled"] = false
			defaultWithChangesXXX, err := yaml.Marshal(defaultProfileKubeletConfig)
			assert.NoError(t, err)

			defaultProfileKubeletConfig = getDefaultProfile(dnsAddr, clientCAFile, volumePluginDir)
			defaultProfileKubeletConfig["authentication"].(map[string]interface{})["webhook"].(map[string]interface{})["cacheTTL"] = "15s"
			defaultWithChangesYYY, err := yaml.Marshal(defaultProfileKubeletConfig)

			assert.NoError(t, err)
			assert.YAMLEq(t, string(defaultWithChangesXXX), profileXXX.Data["kubelet"])
			assert.YAMLEq(t, string(defaultWithChangesYYY), profileYYY.Data["kubelet"])
		})
	})
}

func defaultConfigWithUserProvidedProfiles(t *testing.T) *KubeletConfig {
	k, err := NewKubeletConfig(config.DefaultClusterConfig().Spec, k0sVars)
	assert.NoError(t, err)

	k.clusterSpec.WorkerProfiles = append(k.clusterSpec.WorkerProfiles,
		config.WorkerProfile{
			Name: "profile_XXX",
			Values: map[string]interface{}{
				"authentication": map[string]interface{}{
					"anonymous": map[string]interface{}{
						"enabled": false,
					},
				},
			},
		},
	)

	k.clusterSpec.WorkerProfiles = append(k.clusterSpec.WorkerProfiles,
		config.WorkerProfile{
			Name: "profile_YYY",
			Values: map[string]interface{}{
				"authentication": map[string]interface{}{
					"webhook": map[string]interface{}{
						"cacheTTL": "15s",
					},
				},
			},
		},
	)
	return k
}

func assertConfigMap(t *testing.T, spec string, name string) {
	dst := map[string]interface{}{}
	assert.NoError(t, yaml.Unmarshal([]byte(spec), &dst))

	assert.Equal(t, "ConfigMap", dst["kind"])
	assert.Equal(t, name, dst["metadata"].(map[string]interface{})["name"])
	spec, foundSpec := dst["data"].(map[string]interface{})["kubelet"].(string)
	assert.True(t, foundSpec, "kubelet config map must have embeded kubelet config")
	assert.True(t, strings.TrimSpace(spec) != "", "kubelet config map must have non-empty embeded kubelet config")
}

func assertRole(t *testing.T, spec string, expectedResourceNames []string) {
	dst := map[string]interface{}{}
	assert.NoError(t, yaml.Unmarshal([]byte(spec), &dst))
	assert.Equal(t, "Role", dst["kind"])
	assert.Equal(t, "system:bootstrappers:kubelet-configmaps", dst["metadata"].(map[string]interface{})["name"])
	currentResourceNames := []string{}
	for _, el := range dst["rules"].([]interface{})[0].(map[string]interface{})["resourceNames"].([]interface{}) {
		currentResourceNames = append(currentResourceNames, el.(string))
	}
	assert.Equal(t, expectedResourceNames, currentResourceNames)
}

func assertRoleBinding(t *testing.T, spec string) {
	dst := map[string]interface{}{}
	assert.NoError(t, yaml.Unmarshal([]byte(spec), &dst))
	assert.Equal(t, "RoleBinding", dst["kind"])
	assert.Equal(t, "system:bootstrappers:kubelet-configmaps", dst["metadata"].(map[string]interface{})["name"])
}
