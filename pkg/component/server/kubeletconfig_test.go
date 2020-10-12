package server

import (
	config "github.com/Mirantis/mke/pkg/apis/v1beta1"
	"github.com/ghodss/yaml"
	"github.com/stretchr/testify/assert"
	"strings"
	"testing"
)

func Test_KubeletConfig(t *testing.T) {
	dnsAddr := "dns.local"
	t.Run("default_profile_only", func(t *testing.T) {
		k, err := NewKubeletConfig(config.DefaultClusterConfig().Spec)
		assert.NoError(t, err)
		buf, err := k.run(dnsAddr)
		assert.NoError(t, err)
		manifestYamls := strings.Split(strings.TrimSuffix(buf.String(), "---"), "---")[1:]
		t.Run("output_must_have_3_manifests", func(t *testing.T) {
			assert.Len(t, manifestYamls, 3, "Must have exactly 3 generated manifests per profile")
			assertConfigMap(t, manifestYamls[0], "kubelet-config-default-1.19")
			assertRole(t, manifestYamls[1], "kubelet-config-default-1.19")
			assertRoleBinding(t, manifestYamls[2], "kubelet-config-default-1.19")
		})
	})
	t.Run("with_user_provided_profiles", func(t *testing.T) {
		k := defaultConfigWithUserProvidedProfiles(t)
		buf, err := k.run(dnsAddr)
		assert.NoError(t, err)
		manifestYamls := strings.Split(strings.TrimSuffix(buf.String(), "---"), "---")[1:]
		expectedManifestsCount := 3 * (2 + 1) // 3 manifests per profile, 2 user profiles and 1 default
		assert.Len(t, manifestYamls, expectedManifestsCount, "Must have exactly 3 generated manifests per profile")

		t.Run("final_output_must_have_manifests_for_profiles", func(t *testing.T) {
			// check that each profile has config map, role and role binding
			for idx, profileName := range []string{"default", "profile_XXX", "profile_YYY"} {
				currentProfileOffset := idx * 3
				fullName := "kubelet-config-" + profileName + "-1.19"
				assertConfigMap(t, manifestYamls[currentProfileOffset], fullName)
				assertRole(t, manifestYamls[currentProfileOffset+1], fullName)
				assertRoleBinding(t, manifestYamls[currentProfileOffset+2], fullName)
			}

		})
		t.Run("user_profile_X_must_be_merged_with_default_profile", func(t *testing.T) {

			profileXXX := struct {
				Data map[string]string `yaml:"data"`
			}{}

			profileYYY := struct {
				Data map[string]string `yaml:"data"`
			}{}

			assert.NoError(t, yaml.Unmarshal([]byte(manifestYamls[3]), &profileXXX))
			assert.NoError(t, yaml.Unmarshal([]byte(manifestYamls[6]), &profileYYY))

			// manually apple the same changes to default config and check that there is no diff
			defaultProfileKubeletConfig := getDefaultProfile(dnsAddr)
			defaultProfileKubeletConfig["authentication"].(map[string]interface{})["anonymous"].(map[string]interface{})["enabled"] = false
			defaultWithChangesXXX, err := yaml.Marshal(defaultProfileKubeletConfig)
			assert.NoError(t, err)

			defaultProfileKubeletConfig = getDefaultProfile(dnsAddr)
			defaultProfileKubeletConfig["authentication"].(map[string]interface{})["webhook"].(map[string]interface{})["cacheTTL"] = "15s"
			defaultWithChangesYYY, err := yaml.Marshal(defaultProfileKubeletConfig)

			assert.NoError(t, err)
			assert.YAMLEq(t, string(defaultWithChangesXXX), profileXXX.Data["kubelet"])
			assert.YAMLEq(t, string(defaultWithChangesYYY), profileYYY.Data["kubelet"])
		})
	})
}

func defaultConfigWithUserProvidedProfiles(t *testing.T) *KubeletConfig {
	k, err := NewKubeletConfig(config.DefaultClusterConfig().Spec)
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

	assert.Equal(t, dst["kind"], "ConfigMap")
	assert.Equal(t, dst["metadata"].(map[string]interface{})["name"], name)
	spec, foundSpec := dst["data"].(map[string]interface{})["kubelet"].(string)
	assert.True(t, foundSpec, "kubelet config map must have embeded kubelet config")
	assert.True(t, strings.TrimSpace(spec) != "", "kubelet config map must have non-empty embeded kubelet config")
}

func assertRole(t *testing.T, spec string, name string) {
	dst := map[string]interface{}{}
	assert.NoError(t, yaml.Unmarshal([]byte(spec), &dst))
	assert.Equal(t, dst["kind"], "Role")
	assert.Equal(t, dst["metadata"].(map[string]interface{})["name"], "system:bootstrappers:"+name)
}

func assertRoleBinding(t *testing.T, spec string, name string) {
	dst := map[string]interface{}{}
	assert.NoError(t, yaml.Unmarshal([]byte(spec), &dst))
	assert.Equal(t, dst["kind"], "RoleBinding")
	assert.Equal(t, dst["metadata"].(map[string]interface{})["name"], "system:bootstrappers:"+name)
}