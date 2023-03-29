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

package controller

import (
	"testing"

	"github.com/k0sproject/k0s/pkg/apis/k0s.k0sproject.io/v1beta1"
	"github.com/k0sproject/k0s/pkg/constant"
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
	k0sVars := constant.CfgVars{
		CertRootDir: "/var/lib/k0s/pki",
		EtcdCertDir: "/var/lib/k0s/pki/etcd",
	}

	a.T().Run("internal etcd cluster", func(t *testing.T) {
		storageSpec := &v1beta1.StorageSpec{
			Etcd: &v1beta1.EtcdConfig{
				PeerAddress: "192.168.68.104",
			},
			Type: "etcd",
		}

		result, err := getEtcdArgs(storageSpec, k0sVars)

		a.Nil(err)
		a.Len(result, 4)
		a.Contains(result[0], "--etcd-servers=https://127.0.0.1:2379")
		a.Contains(result[1], "--etcd-cafile=/var/lib/k0s/pki/etcd/ca.crt")
		a.Contains(result[2], "--etcd-certfile=/var/lib/k0s/pki/apiserver-etcd-client.crt")
		a.Contains(result[3], "--etcd-keyfile=/var/lib/k0s/pki/apiserver-etcd-client.key")
	})

	a.T().Run("external etcd cluster with TLS", func(t *testing.T) {
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

		a.Nil(err)
		a.Len(result, 5)
		a.Contains(result[0], "--etcd-servers=https://192.168.10.10:2379,https://192.168.10.11:2379")
		a.Contains(result[1], "--etcd-cafile=/etc/pki/CA/ca.crt")
		a.Contains(result[2], "--etcd-certfile=/etc/pki/tls/certs/etcd-client.crt")
		a.Contains(result[3], "--etcd-keyfile=/etc/pki/tls/private/etcd-client.key")
		a.Contains(result[4], "--etcd-prefix=k0s-tenant-1")
	})

	a.T().Run("external etcd cluster without TLS", func(t *testing.T) {
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

		a.Nil(err)
		a.Len(result, 2)
		a.Contains(result[0], "--etcd-servers=http://192.168.10.10:2379,http://192.168.10.11:2379")
		a.Contains(result[1], "--etcd-prefix=k0s-tenant-1")
	})
}
