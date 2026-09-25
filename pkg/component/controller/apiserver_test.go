// SPDX-FileCopyrightText: 2021 k0s authors
// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/k0sproject/k0s/internal/pkg/stringmap"
	"github.com/k0sproject/k0s/pkg/apis/k0s/v1beta1"
	"github.com/k0sproject/k0s/pkg/config"

	"github.com/stretchr/testify/suite"
)

type apiServerSuite struct {
	suite.Suite
}

func TestApiServerSuite(t *testing.T) {
	apiServerSuite := &apiServerSuite{}

	suite.Run(t, apiServerSuite)
}

func (a *apiServerSuite) TestAuthenticationConfigHasAnonymous() {
	writeConfig := func(content string) string {
		path := filepath.Join(a.T().TempDir(), "authentication-config.yaml")
		a.Require().NoError(os.WriteFile(path, []byte(content), 0o600))
		return path
	}

	a.Run("anonymous field present", func() {
		path := writeConfig(`
apiVersion: apiserver.config.k8s.io/v1beta1
kind: AuthenticationConfiguration
anonymous:
  enabled: true
  conditions:
    - path: /readyz
`)
		hasAnonymous, err := authenticationConfigHasAnonymous(path)
		a.Require().NoError(err)
		a.Require().True(hasAnonymous)
	})

	a.Run("anonymous field present but disabled", func() {
		path := writeConfig(`
apiVersion: apiserver.config.k8s.io/v1beta1
kind: AuthenticationConfiguration
anonymous:
  enabled: false
`)
		hasAnonymous, err := authenticationConfigHasAnonymous(path)
		a.Require().NoError(err)
		a.Require().True(hasAnonymous)
	})

	a.Run("anonymous field absent", func() {
		path := writeConfig(`
apiVersion: apiserver.config.k8s.io/v1beta1
kind: AuthenticationConfiguration
jwt:
  - issuer:
      url: https://example.com/dex
`)
		hasAnonymous, err := authenticationConfigHasAnonymous(path)
		a.Require().NoError(err)
		a.Require().False(hasAnonymous)
	})

	a.Run("anonymous field null", func() {
		path := writeConfig(`
apiVersion: apiserver.config.k8s.io/v1beta1
kind: AuthenticationConfiguration
anonymous: null
`)
		hasAnonymous, err := authenticationConfigHasAnonymous(path)
		a.Require().NoError(err)
		a.Require().False(hasAnonymous)
	})

	a.Run("file missing", func() {
		hasAnonymous, err := authenticationConfigHasAnonymous(filepath.Join(a.T().TempDir(), "nonexistent.yaml"))
		a.Require().Error(err)
		a.Require().False(hasAnonymous)
	})

	a.Run("file malformed", func() {
		path := writeConfig(`{unparseable`)
		hasAnonymous, err := authenticationConfigHasAnonymous(path)
		a.Require().Error(err)
		a.Require().False(hasAnonymous)
	})
}

func (a *apiServerSuite) TestAddEtcdArgs() {
	k0sVars := &config.CfgVars{
		KineSocketPath: "/run/k0s/kine/kine.sock:2379",
		CertRootDir:    "/var/lib/k0s/pki",
		EtcdCertDir:    "/var/lib/k0s/pki/etcd",
	}

	for _, tt := range []struct {
		name     string
		storage  *v1beta1.StorageSpec
		expected stringmap.StringMap
	}{
		{
			"kine",
			&v1beta1.StorageSpec{
				Type: "kine",
				Kine: v1beta1.DefaultKineConfig("/var/lib/k0s"),
			},
			stringmap.StringMap{
				"etcd-servers": "unix:/run/k0s/kine/kine.sock:2379",
			},
		},
		{
			"internal etcd cluster",
			&v1beta1.StorageSpec{
				Type: "etcd",
				Etcd: &v1beta1.EtcdConfig{
					PeerAddress: "192.168.68.104",
				},
			},
			stringmap.StringMap{
				"etcd-servers":  "https://127.0.0.1:2379",
				"etcd-cafile":   filepath.FromSlash("/var/lib/k0s/pki/etcd/ca.crt"),
				"etcd-certfile": filepath.FromSlash("/var/lib/k0s/pki/apiserver-etcd-client.crt"),
				"etcd-keyfile":  filepath.FromSlash("/var/lib/k0s/pki/apiserver-etcd-client.key"),
			},
		},
		{
			"external etcd cluster with TLS",
			&v1beta1.StorageSpec{
				Type: "etcd",
				Etcd: &v1beta1.EtcdConfig{
					ExternalCluster: &v1beta1.ExternalCluster{
						Endpoints:      []string{"https://192.168.10.10:2379", "https://192.168.10.11:2379"},
						EtcdPrefix:     "k0s-tenant-1",
						CaFile:         "/etc/pki/CA/ca.crt",
						ClientCertFile: "/etc/pki/tls/certs/etcd-client.crt",
						ClientKeyFile:  "/etc/pki/tls/private/etcd-client.key",
					},
				},
			},
			stringmap.StringMap{
				"etcd-servers":  "https://192.168.10.10:2379,https://192.168.10.11:2379",
				"etcd-cafile":   "/etc/pki/CA/ca.crt",
				"etcd-certfile": "/etc/pki/tls/certs/etcd-client.crt",
				"etcd-keyfile":  "/etc/pki/tls/private/etcd-client.key",
				"etcd-prefix":   "k0s-tenant-1",
			},
		},
		{
			"external etcd cluster without TLS",
			&v1beta1.StorageSpec{
				Type: "etcd",
				Etcd: &v1beta1.EtcdConfig{
					ExternalCluster: &v1beta1.ExternalCluster{
						Endpoints:  []string{"http://192.168.10.10:2379", "http://192.168.10.11:2379"},
						EtcdPrefix: "k0s-tenant-1",
					},
				},
			},
			stringmap.StringMap{
				"etcd-servers": "http://192.168.10.10:2379,http://192.168.10.11:2379",
				"etcd-prefix":  "k0s-tenant-1",
			},
		},
	} {
		a.Run(tt.name, func() {
			args := make(stringmap.StringMap)
			a.Require().NoError(addEtcdArgs(args, tt.storage, k0sVars))
			a.Equal(tt.expected, args)
		})
	}

	a.Run("invalid storage type", func() {
		storageSpec := &v1beta1.StorageSpec{Type: "bogus"}

		args := make(stringmap.StringMap)
		a.ErrorContains(addEtcdArgs(args, storageSpec, k0sVars), "invalid storage type: bogus")
		a.Empty(args)
	})
}

func (a *apiServerSuite) TestAPIServer_BuildConfig() {
	newAPIServer := func() *APIServer {
		return &APIServer{
			NodeConfig: v1beta1.DefaultClusterConfig(),
			K0sVars: &config.CfgVars{
				CertRootDir:           "/var/lib/k0s/pki",
				DataDir:               "/var/lib/k0s",
				EtcdCertDir:           "/var/lib/k0s/pki/etcd",
				KonnectivitySocketDir: "/run/k0s/konnectivity-server",
			},
			LogLevel: "1",
		}
	}

	build := func(underTest *APIServer) *apiServerConfig {
		cfg, err := underTest.buildConfig()
		a.Require().NoError(err)
		return cfg
	}

	a.Run("konnectivity disabled", func() {
		underTest := newAPIServer()

		cfg := build(underTest)
		a.Nil(cfg.egressSelector)
		a.NotContains(cfg.flags, "egress-selector-config-file")
		a.Equal("https://kubernetes.default.svc", cfg.flags["api-audiences"])
	})

	a.Run("konnectivity enabled", func() {
		underTest := newAPIServer()
		underTest.EnableKonnectivity = true

		cfg := build(underTest)
		a.Equal(&egressSelectorConfig{
			Path:    filepath.FromSlash("/var/lib/k0s/konnectivity.conf"),
			UDSName: filepath.FromSlash("/run/k0s/konnectivity-server/konnectivity-server.sock"),
		}, cfg.egressSelector)
		a.Equal(cfg.egressSelector.Path, cfg.flags["egress-selector-config-file"])
		a.Equal("https://kubernetes.default.svc,system:konnectivity-server", cfg.flags["api-audiences"])
	})

	a.Run("extra args override flags", func() {
		underTest := newAPIServer()
		underTest.NodeConfig.Spec.API.ExtraArgs = map[string]string{
			"authorization-mode": "AlwaysAllow",
			"etcd-servers":       "https://etcd.example.com:2379",
			"custom-flag":        "custom-value",
		}

		cfg := build(underTest)
		a.Equal("AlwaysAllow", cfg.flags["authorization-mode"])
		a.Equal("https://etcd.example.com:2379", cfg.flags["etcd-servers"])
		a.Equal("custom-value", cfg.flags["custom-flag"])
	})

	a.Run("raw args are passed through", func() {
		underTest := newAPIServer()
		underTest.NodeConfig.Spec.API.RawArgs = []string{"--foo", "--bar=baz"}

		cfg := build(underTest)
		a.Equal([]string{"--foo", "--bar=baz"}, cfg.rawArgs)
	})

	a.Run("bind address", func() {
		underTest := newAPIServer()
		underTest.NodeConfig.Spec.API.Address = "192.0.2.1"

		cfg := build(underTest)
		a.NotContains(cfg.flags, "bind-address")

		underTest.NodeConfig.Spec.API.OnlyBindToAddress = true

		cfg = build(underTest)
		a.Equal("192.0.2.1", cfg.flags["bind-address"])
	})

	a.Run("endpoint reconciler disabled", func() {
		underTest := newAPIServer()

		cfg := build(underTest)
		a.NotContains(cfg.flags, "endpoint-reconciler-type")

		underTest.DisableEndpointReconciler = true

		cfg = build(underTest)
		a.Equal("none", cfg.flags["endpoint-reconciler-type"])
	})

	a.Run("anonymous auth", func() {
		writeAuthConfig := func(content string) string {
			path := filepath.Join(a.T().TempDir(), "authentication-config.yaml")
			a.Require().NoError(os.WriteFile(path, []byte(content), 0o600))
			return path
		}

		a.Run("defaults to false", func() {
			cfg := build(newAPIServer())
			a.Equal("false", cfg.flags["anonymous-auth"])
		})

		a.Run("not set if managed via authentication config", func() {
			underTest := newAPIServer()
			underTest.NodeConfig.Spec.API.ExtraArgs = map[string]string{
				"authentication-config": writeAuthConfig("anonymous: {enabled: false}\n"),
			}

			cfg := build(underTest)
			a.NotContains(cfg.flags, "anonymous-auth")
		})

		a.Run("defaults to false if not managed via authentication config", func() {
			underTest := newAPIServer()
			underTest.NodeConfig.Spec.API.ExtraArgs = map[string]string{
				"authentication-config": writeAuthConfig("jwt: []\n"),
			}

			cfg := build(underTest)
			a.Equal("false", cfg.flags["anonymous-auth"])
		})

		a.Run("defaults to false if authentication config is unreadable", func() {
			underTest := newAPIServer()
			underTest.NodeConfig.Spec.API.ExtraArgs = map[string]string{
				"authentication-config": filepath.Join(a.T().TempDir(), "nonexistent.yaml"),
			}

			cfg := build(underTest)
			a.Equal("false", cfg.flags["anonymous-auth"])
		})
	})

	a.Run("stop timeout", func() {
		for _, tt := range []struct {
			name           string
			configured     time.Duration
			requestTimeout string // request-timeout extra arg, if any
			watchGrace     string // shutdown-watch-termination-grace-period extra arg, if any
			expected       time.Duration
			expectedGrace  string
		}{
			{"uses configured value", 42 * time.Second, "", "", 42 * time.Second, "40s"},
			{"clamps default request timeout", 0, "", "", 20 * time.Second, "18s"},
			{"derives from request timeout", 0, "10s", "", 12 * time.Second, "10s"},
			{"clamps short request timeout", 0, "1s", "", 5 * time.Second, "3s"},
			{"keeps user-provided watch termination grace period", 0, "1s", "7s", 9 * time.Second, "7s"},
		} {
			a.Run(tt.name, func() {
				underTest := newAPIServer()
				underTest.StopTimeout = tt.configured
				underTest.NodeConfig.Spec.API.ExtraArgs = map[string]string{}
				if tt.requestTimeout != "" {
					underTest.NodeConfig.Spec.API.ExtraArgs["request-timeout"] = tt.requestTimeout
				}
				if tt.watchGrace != "" {
					underTest.NodeConfig.Spec.API.ExtraArgs["shutdown-watch-termination-grace-period"] = tt.watchGrace
				}

				cfg := build(underTest)
				a.Equal(tt.expected, cfg.stopTimeout)
				a.Equal(tt.expectedGrace, cfg.flags["shutdown-watch-termination-grace-period"])
			})
		}
	})

	a.Run("invalid storage type", func() {
		underTest := newAPIServer()
		underTest.NodeConfig.Spec.Storage.Type = "bogus"

		cfg, err := underTest.buildConfig()
		a.ErrorContains(err, "invalid storage type: bogus")
		a.Nil(cfg)
	})
}

func (a *apiServerSuite) TestAPIServerConfig_WriteFiles() {
	a.Run("nothing to write", func() {
		a.NoError((&apiServerConfig{}).writeFiles())
	})

	a.Run("egress selector config", func() {
		path := filepath.Join(a.T().TempDir(), "konnectivity.conf")
		cfg := &apiServerConfig{
			egressSelector: &egressSelectorConfig{
				Path:    path,
				UDSName: "/run/k0s/konnectivity-server/konnectivity-server.sock",
			},
		}

		a.Require().NoError(cfg.writeFiles())

		content, err := os.ReadFile(path)
		a.Require().NoError(err)
		a.Contains(string(content), "kind: EgressSelectorConfiguration")
		a.Contains(string(content), "udsName: /run/k0s/konnectivity-server/konnectivity-server.sock")
	})
}
