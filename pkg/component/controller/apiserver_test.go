// SPDX-FileCopyrightText: 2021 k0s authors
// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"os"
	"path/filepath"
	"testing"

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

func (a *apiServerSuite) TestCapNetBindServiceForLowPorts() {
	k0sVars := &config.CfgVars{
		BinDir:      "/var/lib/k0s/bin",
		CertRootDir: "/var/lib/k0s/pki",
		DataDir:     "/var/lib/k0s",
		RunDir:      "/run/k0s",
	}

	a.Run("port 443 requires CAP_NET_BIND_SERVICE", func() {
		clusterConfig := v1beta1.DefaultClusterConfig()
		clusterConfig.Spec.API.Port = 443

		apiServer := &APIServer{
			NodeConfig:     clusterConfig,
			K0sVars:        k0sVars,
			LogLevel:       "1",
			executablePath: "/fake/path/kube-apiserver",
		}

		supervisor, err := apiServer.buildSupervisor()
		require := a.Require()
		require.NoError(err)
		require.True(supervisor.RequiredPrivileges.BindsPrivilegedPorts,
			"Port 443 should require CAP_NET_BIND_SERVICE capability")
	})

	a.Run("port 6443 does not require CAP_NET_BIND_SERVICE", func() {
		clusterConfig := v1beta1.DefaultClusterConfig()
		clusterConfig.Spec.API.Port = 6443

		apiServer := &APIServer{
			NodeConfig:     clusterConfig,
			K0sVars:        k0sVars,
			LogLevel:       "1",
			executablePath: "/fake/path/kube-apiserver",
		}

		supervisor, err := apiServer.buildSupervisor()
		require := a.Require()
		require.NoError(err)
		require.False(supervisor.RequiredPrivileges.BindsPrivilegedPorts,
			"Port 6443 should not require CAP_NET_BIND_SERVICE capability")
	})

	a.Run("port 80 requires CAP_NET_BIND_SERVICE", func() {
		clusterConfig := v1beta1.DefaultClusterConfig()
		clusterConfig.Spec.API.Port = 80

		apiServer := &APIServer{
			NodeConfig:     clusterConfig,
			K0sVars:        k0sVars,
			LogLevel:       "1",
			executablePath: "/fake/path/kube-apiserver",
		}

		supervisor, err := apiServer.buildSupervisor()
		require := a.Require()
		require.NoError(err)
		require.True(supervisor.RequiredPrivileges.BindsPrivilegedPorts,
			"Port 80 should require CAP_NET_BIND_SERVICE capability")
	})
}
