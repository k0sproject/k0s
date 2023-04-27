/*
Copyright 2020 k0s authors

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

package controller

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/k0sproject/k0s/internal/testutil"

	helmv1beta1 "github.com/k0sproject/k0s/pkg/apis/helm/v1beta1"
	"github.com/k0sproject/k0s/pkg/apis/k0s/v1beta1"
	"github.com/k0sproject/k0s/pkg/config"

	"github.com/stretchr/testify/require"
	"sigs.k8s.io/yaml"
)

func Test_KubeletConfig(t *testing.T) {
	cfg := v1beta1.DefaultClusterConfig()
	k0sVars, err := config.NewCfgVars(nil, t.TempDir())
	require.NoError(t, err)
	dnsAddr, _ := cfg.Spec.Network.DNSAddress()
	t.Run("default_profile_only", func(t *testing.T) {
		k := NewKubeletConfig(k0sVars, testutil.NewFakeClientFactory(), cfg)

		t.Log("starting to run...")
		buf, err := k.createProfiles(cfg)
		require.NoError(t, err)
		if err != nil {
			t.FailNow()
		}
		manifestYamls := strings.Split(strings.TrimSuffix(buf.String(), "---"), "---")[1:]
		t.Run("output_must_have_3_manifests", func(t *testing.T) {
			require.Len(t, manifestYamls, 4, "Must have exactly 4 generated manifests per profile")
			requireConfigMap(t, manifestYamls[0], "kubelet-config-default-1.27")
			requireConfigMap(t, manifestYamls[1], "kubelet-config-default-windows-1.27")
			requireRole(t, manifestYamls[2], []string{
				formatProfileName("default"),
				formatProfileName("default-windows"),
			})
			requireRoleBinding(t, manifestYamls[3])
		})
	})
	t.Run("default_profile_must_pass_down_cluster_domain", func(t *testing.T) {
		profile := getDefaultProfile(dnsAddr, "cluster.local.custom")
		require.Equal(t, string(
			"cluster.local.custom",
		), profile["clusterDomain"])
	})
	t.Run("with_user_provided_profiles", func(t *testing.T) {
		k, cfgWithUserProvidedProfiles := defaultConfigWithUserProvidedProfiles(t)
		buf, err := k.createProfiles(cfgWithUserProvidedProfiles)
		require.NoError(t, err)
		manifestYamls := strings.Split(strings.TrimSuffix(buf.String(), "---"), "---")[1:]
		expectedManifestsCount := 6
		require.Len(t, manifestYamls, expectedManifestsCount, "Must have exactly 6 generated manifests per profile")

		t.Run("final_output_must_have_manifests_for_profiles", func(t *testing.T) {
			// check that each profile has config map, role and role binding
			var resourceNamesForRole []string
			for idx, profileName := range []string{"default", "default-windows", "profile_XXX", "profile_YYY"} {
				fullName := "kubelet-config-" + profileName + "-1.27"
				resourceNamesForRole = append(resourceNamesForRole, formatProfileName(profileName))
				requireConfigMap(t, manifestYamls[idx], fullName)
			}
			requireRole(t, manifestYamls[len(resourceNamesForRole)], resourceNamesForRole)
		})
		t.Run("user_profile_X_must_be_merged_with_default_profile", func(t *testing.T) {
			profileXXX := struct {
				Data map[string]string `yaml:"data"`
			}{}

			profileYYY := struct {
				Data map[string]string `yaml:"data"`
			}{}

			require.NoError(t, yaml.Unmarshal([]byte(manifestYamls[2]), &profileXXX))
			require.NoError(t, yaml.Unmarshal([]byte(manifestYamls[3]), &profileYYY))

			// manually apple the same changes to default config and check that there is no diff
			defaultProfileKubeletConfig := getDefaultProfile(dnsAddr, "cluster.local")
			defaultProfileKubeletConfig["authentication"] = map[string]interface{}{
				"anonymous": map[string]interface{}{
					"enabled": false,
				},
			}
			defaultWithChangesXXX, err := yaml.Marshal(defaultProfileKubeletConfig)
			require.NoError(t, err)

			defaultProfileKubeletConfig = getDefaultProfile(dnsAddr, "cluster.local")
			defaultProfileKubeletConfig["authentication"] = map[string]interface{}{
				"webhook": map[string]interface{}{
					"cacheTTL": "15s",
				},
			}
			defaultWithChangesYYY, err := yaml.Marshal(defaultProfileKubeletConfig)

			require.NoError(t, err)

			require.YAMLEq(t, string(defaultWithChangesXXX), profileXXX.Data["kubelet"])
			require.YAMLEq(t, string(defaultWithChangesYYY), profileYYY.Data["kubelet"])
		})
	})
}

func defaultConfigWithUserProvidedProfiles(t *testing.T) (*KubeletConfig, *v1beta1.ClusterConfig) {
	cfg := v1beta1.DefaultClusterConfig()
	k0sVars, err := config.NewCfgVars(nil, t.TempDir())
	require.NoError(t, err)
	k := NewKubeletConfig(k0sVars, testutil.NewFakeClientFactory(), cfg)

	cfgProfileX := map[string]interface{}{
		"authentication": map[string]interface{}{
			"anonymous": map[string]interface{}{
				"enabled": false,
			},
		},
	}
	wcx, err := json.Marshal(cfgProfileX)
	if err != nil {
		t.Fatal(err)
	}
	cfg.Spec.WorkerProfiles = append(cfg.Spec.WorkerProfiles,
		v1beta1.WorkerProfile{
			Name:   "profile_XXX",
			Config: wcx,
		},
	)

	cfgProfileY := map[string]interface{}{
		"authentication": map[string]interface{}{
			"webhook": map[string]interface{}{
				"cacheTTL": "15s",
			},
		},
	}

	wcy, err := json.Marshal(cfgProfileY)
	if err != nil {
		t.Fatal(err)
	}

	cfg.Spec.WorkerProfiles = append(cfg.Spec.WorkerProfiles,
		v1beta1.WorkerProfile{
			Name:   "profile_YYY",
			Config: wcy,
		},
	)
	return k, cfg
}

func requireConfigMap(t *testing.T, spec string, name string) {
	dst := map[string]interface{}{}
	require.NoError(t, yaml.Unmarshal([]byte(spec), &dst))
	dst = helmv1beta1.CleanUpGenericMap(dst)
	require.Equal(t, "ConfigMap", dst["kind"])
	require.Equal(t, name, dst["metadata"].(map[string]interface{})["name"])
	spec, foundSpec := dst["data"].(map[string]interface{})["kubelet"].(string)
	require.True(t, foundSpec, "kubelet config map must have embedded kubelet config")
	require.True(t, strings.TrimSpace(spec) != "", "kubelet config map must have non-empty embedded kubelet config")
}

func requireRole(t *testing.T, spec string, expectedResourceNames []string) {
	dst := map[string]interface{}{}
	require.NoError(t, yaml.Unmarshal([]byte(spec), &dst))
	dst = helmv1beta1.CleanUpGenericMap(dst)
	require.Equal(t, "Role", dst["kind"])
	require.Equal(t, "system:bootstrappers:kubelet-configmaps", dst["metadata"].(map[string]interface{})["name"])
	var currentResourceNames []string
	for _, el := range dst["rules"].([]interface{})[0].(map[string]interface{})["resourceNames"].([]interface{}) {
		currentResourceNames = append(currentResourceNames, el.(string))
	}
	require.Equal(t, expectedResourceNames, currentResourceNames)
}

func requireRoleBinding(t *testing.T, spec string) {
	dst := map[string]interface{}{}
	require.NoError(t, yaml.Unmarshal([]byte(spec), &dst))
	dst = helmv1beta1.CleanUpGenericMap(dst)
	require.Equal(t, "RoleBinding", dst["kind"])
	require.Equal(t, "system:bootstrappers:kubelet-configmaps", dst["metadata"].(map[string]interface{})["name"])
}
