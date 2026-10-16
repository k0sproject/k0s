// SPDX-FileCopyrightText: 2026 k0s authors
// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"testing"

	"github.com/k0sproject/k0s/pkg/apis/k0s/v1beta1"
	"github.com/k0sproject/k0s/pkg/config"
	"github.com/k0sproject/k0s/pkg/constant"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	apiserverv1 "k8s.io/apiserver/pkg/apis/apiserver/v1"
	"sigs.k8s.io/yaml"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestManageDiscoveryAuthentication(t *testing.T) {
	for _, test := range []struct {
		args    map[string]string
		name    string
		rawArgs []string
		want    bool
	}{
		{map[string]string{"enable-bootstrap-token-auth": "true"}, "defaults", nil, true},
		{map[string]string{"authentication-config": "/etc/authn.yaml"}, "user authentication config", nil, false},
		{map[string]string{"anonymous-auth": "true"}, "explicit anonymous auth", nil, false},
		{map[string]string{"oidc-issuer-url": "https://issuer"}, "oidc flags", nil, false},
		{map[string]string{"oidc-client-id": "k0s"}, "any oidc flag", nil, false},
		{map[string]string{"anonymous-auth": "false"}, "anonymous auth explicitly off", nil, false},
		{map[string]string{"authentication-config": ""}, "empty authentication config", nil, true},
		{map[string]string{"authorization-config": "/etc/authz.yaml"}, "authorization config only", nil, true},
		{map[string]string{"service-account-issuer": "https://issuer"}, "unrelated issuer flag", nil, true},
		{nil, "no arguments", nil, true},

		// Raw arguments are passed on as they are, after the ones above.
		{nil, "raw authentication config", []string{"--authentication-config=/etc/authn.yaml"}, false},
		{nil, "raw authentication config in two arguments", []string{"--authentication-config", "/etc/authn.yaml"}, false},
		{nil, "raw authentication config with one dash", []string{"-authentication-config=/etc/authn.yaml"}, false},
		{nil, "raw anonymous auth", []string{"--anonymous-auth=false"}, false},
		{nil, "raw anonymous auth without value", []string{"--anonymous-auth"}, false},
		{nil, "raw oidc flags", []string{"--oidc-issuer-url=https://issuer", "--oidc-client-id=k0s"}, false},
		{nil, "raw oidc flag in two arguments", []string{"--oidc-username-claim", "email"}, false},
		{nil, "values aren't flags", []string{"--service-account-issuer", "oidc-like-value"}, true},
		{nil, "unrelated raw arguments", []string{"--v=2", "--audit-log-path=-"}, true},
	} {
		t.Run(test.name, func(t *testing.T) {
			assert.Equal(t, test.want, manageDiscoveryAuthentication(test.args, test.rawArgs))
		})
	}
}

func TestWriteDiscoveryAuthenticationConfig(t *testing.T) {
	k0sVars, err := config.NewCfgVars(nil, t.TempDir())
	require.NoError(t, err)
	require.NoError(t, os.MkdirAll(k0sVars.DataDir, 0o700))

	path := discoveryAuthenticationConfigPath(k0sVars)
	assert.Equal(t, filepath.Join(k0sVars.DataDir, "authentication-config.yaml"), path)
	require.NoError(t, writeDiscoveryAuthenticationConfig(path))

	// The existing check finds the anonymous field, so k0s doesn't pass --anonymous-auth as well.
	hasAnonymous, err := authenticationConfigHasAnonymous(path)
	require.NoError(t, err)
	assert.True(t, hasAnonymous)

	// Anonymous requests authenticate for the cluster-info only, nothing else is configured.
	data, err := os.ReadFile(filepath.Clean(path))
	require.NoError(t, err)
	var authn apiserverv1.AuthenticationConfiguration
	require.NoError(t, yaml.UnmarshalStrict(data, &authn))
	assert.Equal(t, apiserverv1.AuthenticationConfiguration{
		TypeMeta: metav1.TypeMeta{APIVersion: "apiserver.config.k8s.io/v1", Kind: "AuthenticationConfiguration"},
		Anonymous: &apiserverv1.AnonymousAuthConfig{
			Enabled:    true,
			Conditions: []apiserverv1.AnonymousAuthCondition{{Path: "/api/v1/namespaces/kube-public/configmaps/cluster-info"}},
		},
	}, authn)
	// Windows has no Unix permissions.
	if runtime.GOOS != "windows" {
		var info os.FileInfo
		info, err = os.Stat(path)
		require.NoError(t, err)
		assert.Equal(t, os.FileMode(constant.CertMode), info.Mode().Perm(), "readable for the non-root kube-apiserver")
	}
}

func TestClusterInfoDiscovery(t *testing.T) {
	for _, enabled := range []bool{false, true} {
		t.Run(map[bool]string{false: "disabled", true: "enabled"}[enabled], func(t *testing.T) {
			k0sVars, err := config.NewCfgVars(nil, t.TempDir())
			require.NoError(t, err)
			require.NoError(t, os.MkdirAll(k0sVars.DataDir, 0o700))
			nodeConfig := v1beta1.DefaultClusterConfig()
			nodeConfig.Spec.API.ClusterInfoDiscovery = enabled

			cfg, err := (&APIServer{NodeConfig: nodeConfig, K0sVars: k0sVars, LogLevel: "1"}).buildConfig()
			require.NoError(t, err)
			path := filepath.Join(k0sVars.DataDir, "authentication-config.yaml")
			args := cfg.flags.ToDashedArgs()
			assert.Equal(t, enabled, slices.Contains(args, "--authentication-config="+path), "%v", args)
			// Without it, k0s keeps anonymous authentication off.
			assert.Equal(t, !enabled, slices.Contains(args, "--anonymous-auth=false"), "%v", args)

			// Building the config doesn't write anything, the file comes with the other files of kube-apiserver.
			assert.NoFileExists(t, path)
			require.NoError(t, cfg.writeFiles())
			if enabled {
				assert.FileExists(t, path)
			} else {
				assert.NoFileExists(t, path)
			}
		})
	}
}
