// SPDX-FileCopyrightText: 2021 k0s authors
// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"path/filepath"
	"testing"

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

func (a *apiServerSuite) TestGetEtcdArgs() {
	k0sVars := &config.CfgVars{
		KineSocketPath: "/run/k0s/kine/kine.sock:2379",
		CertRootDir:    "/var/lib/k0s/pki",
		EtcdCertDir:    "/var/lib/k0s/pki/etcd",
	}

	a.Run("kine", func() {
		storageSpec := &v1beta1.StorageSpec{
			Type: "kine",
			Kine: v1beta1.DefaultKineConfig("/var/lib/k0s"),
		}

		result, err := getEtcdArgs(storageSpec, k0sVars)

		require := a.Require()
		require.NoError(err)
		require.Len(result, 1)
		require.Contains(result[0], "--etcd-servers=unix:/run/k0s/kine/kine.sock:2379")
	})

	a.Run("internal etcd cluster", func() {
		storageSpec := &v1beta1.StorageSpec{
			Etcd: &v1beta1.EtcdConfig{
				PeerAddress: "192.168.68.104",
			},
			Type: "etcd",
		}

		result, err := getEtcdArgs(storageSpec, k0sVars)

		require := a.Require()
		require.NoError(err)
		require.Len(result, 4)
		require.Contains(result[0], "--etcd-servers=https://127.0.0.1:2379")
		require.Contains(result[1], "--etcd-cafile="+filepath.FromSlash("/var/lib/k0s/pki/etcd/ca.crt"))
		require.Contains(result[2], "--etcd-certfile="+filepath.FromSlash("/var/lib/k0s/pki/apiserver-etcd-client.crt"))
		require.Contains(result[3], "--etcd-keyfile="+filepath.FromSlash("/var/lib/k0s/pki/apiserver-etcd-client.key"))
	})

	a.Run("external etcd cluster with TLS", func() {
		storageSpec := &v1beta1.StorageSpec{
			Etcd: &v1beta1.EtcdConfig{
				ExternalCluster: &v1beta1.ExternalCluster{
					Endpoints:      []string{"https://192.168.10.10:2379", "https://192.168.10.11:2379"},
					EtcdPrefix:     "k0s-tenant-1",
					CaFile:         "/etc/pki/CA/ca.crt",
					ClientCertFile: "/etc/pki/tls/certs/etcd-client.crt",
					ClientKeyFile:  "/etc/pki/tls/private/etcd-client.key",
				},
			},
			Type: "etcd",
		}

		result, err := getEtcdArgs(storageSpec, k0sVars)

		require := a.Require()
		require.NoError(err)
		require.Len(result, 5)
		require.Contains(result[0], "--etcd-servers=https://192.168.10.10:2379,https://192.168.10.11:2379")
		require.Contains(result[1], "--etcd-cafile=/etc/pki/CA/ca.crt")
		require.Contains(result[2], "--etcd-certfile=/etc/pki/tls/certs/etcd-client.crt")
		require.Contains(result[3], "--etcd-keyfile=/etc/pki/tls/private/etcd-client.key")
		require.Contains(result[4], "--etcd-prefix=k0s-tenant-1")
	})

	a.Run("external etcd cluster without TLS", func() {
		storageSpec := &v1beta1.StorageSpec{
			Etcd: &v1beta1.EtcdConfig{
				ExternalCluster: &v1beta1.ExternalCluster{
					Endpoints:  []string{"http://192.168.10.10:2379", "http://192.168.10.11:2379"},
					EtcdPrefix: "k0s-tenant-1",
				},
			},
			Type: "etcd",
		}

		result, err := getEtcdArgs(storageSpec, k0sVars)

		require := a.Require()
		require.NoError(err)
		require.Len(result, 2)
		require.Contains(result[0], "--etcd-servers=http://192.168.10.10:2379,http://192.168.10.11:2379")
		require.Contains(result[1], "--etcd-prefix=k0s-tenant-1")
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
			ClusterConfig:  clusterConfig,
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
			ClusterConfig:  clusterConfig,
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
			ClusterConfig:  clusterConfig,
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
