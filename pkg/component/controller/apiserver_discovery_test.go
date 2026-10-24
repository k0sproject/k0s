// SPDX-FileCopyrightText: 2026 k0s authors
// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/k0sproject/k0s/pkg/apis/k0s/v1beta1"
	"github.com/k0sproject/k0s/pkg/config"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestManageDiscoveryAuthentication(t *testing.T) {
	for _, test := range []struct {
		name    string
		args    map[string]string
		rawArgs []string
		want    bool
	}{
		{"no arguments", nil, nil, true},
		{"unrelated arguments", map[string]string{"some-flag": "some-value"}, []string{"--other-flag=other-value"}, true},
		{"authentication config", map[string]string{"authentication-config": "/made/up.yaml"}, nil, false},
		{"empty authentication config", map[string]string{"authentication-config": ""}, nil, true},
		{"anonymous auth", map[string]string{"anonymous-auth": "true"}, nil, false},
		{"OIDC flag", map[string]string{"oidc-issuer-url": "https://issuer.example.com"}, nil, false},
		{"empty OIDC flag", map[string]string{"oidc-client-id": ""}, nil, false},
		{"raw authentication config", nil, []string{"--authentication-config=/made/up.yaml"}, false},
		{"raw authentication config with separate value", nil, []string{"--authentication-config", "/made/up.yaml"}, false},
		{"raw anonymous auth without value", nil, []string{"-anonymous-auth"}, false},
		{"raw OIDC flag", nil, []string{"--oidc-username-claim", "email"}, false},
		{"raw value that looks like a flag name", nil, []string{"--some-flag", "oidc-issuer-url"}, true},
	} {
		t.Run(test.name, func(t *testing.T) {
			assert.Equal(t, test.want, manageDiscoveryAuthentication(test.args, test.rawArgs),
				"k0s should manage the authentication config only if users don't configure authentication")
		})
	}
}

func TestAPIServer_ClusterInfoDiscovery(t *testing.T) {
	for _, enabled := range []bool{false, true} {
		t.Run(map[bool]string{false: "disabled", true: "enabled"}[enabled], func(t *testing.T) {
			k0sVars, err := config.NewCfgVars(nil, t.TempDir())
			require.NoError(t, err)
			require.NoError(t, os.MkdirAll(k0sVars.DataDir, 0o700))
			nodeConfig := v1beta1.DefaultClusterConfig()
			nodeConfig.Spec.API.ClusterInfoDiscovery = enabled

			cfg, err := (&APIServer{NodeConfig: nodeConfig, K0sVars: k0sVars, LogLevel: "1"}).buildConfig()
			require.NoError(t, err)
			require.NoError(t, cfg.writeFiles())

			path := filepath.Join(k0sVars.DataDir, "authentication-config.yaml")
			if !enabled {
				assert.NotContains(t, cfg.flags, "authentication-config", "no authentication config should be passed")
				assert.Equal(t, "false", cfg.flags["anonymous-auth"], "anonymous auth should stay off")
				assert.NoFileExists(t, path, "no authentication config should be written")
				return
			}

			assert.Equal(t, path, cfg.flags["authentication-config"], "the written authentication config should be passed")
			assert.NotContains(t, cfg.flags, "anonymous-auth", "anonymous auth should be left to the authentication config")
			hasAnonymous, err := authenticationConfigHasAnonymous(path)
			require.NoError(t, err)
			assert.True(t, hasAnonymous, "the authentication config should configure anonymous auth")
			data, err := os.ReadFile(path)
			require.NoError(t, err)
			assert.Contains(t, string(data), "path: /api/v1/namespaces/kube-public/configmaps/cluster-info",
				"anonymous requests should be limited to the cluster-info")
		})
	}
}
