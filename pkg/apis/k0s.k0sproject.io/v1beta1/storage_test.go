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

package v1beta1

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

func TestStorageSpec_IsJoinable(t *testing.T) {
	tests := []struct {
		name    string
		storage StorageSpec
		want    bool
	}{
		{
			name: "etcd",
			storage: StorageSpec{
				Type: "etcd",
			},
			want: true,
		},
		{
			name: "etcd",
			storage: StorageSpec{
				Type: "etcd",
				Etcd: &EtcdConfig{
					ExternalCluster: &ExternalCluster{
						Endpoints:  []string{"https://192.168.10.2:2379"},
						EtcdPrefix: "k0s-tenant-1",
					},
					PeerAddress: "",
				},
			},
			want: false,
		},
		{
			name: "kine-sqlite",
			storage: StorageSpec{
				Type: "kine",
				Kine: &KineConfig{
					DataSource: "sqlite://foobar",
				},
			},
			want: false,
		},
		{
			name: "kine-mysql",
			storage: StorageSpec{
				Type: "kine",
				Kine: &KineConfig{
					DataSource: "mysql://foobar",
				},
			},
			want: true,
		},
		{
			name: "kine-postgres",
			storage: StorageSpec{
				Type: "kine",
				Kine: &KineConfig{
					DataSource: "postgres://foobar",
				},
			},
			want: true,
		},
		{
			name: "kine-unknown",
			storage: StorageSpec{
				Type: "kine",
				Kine: &KineConfig{
					DataSource: "unknown://foobar",
				},
			},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.storage.IsJoinable(); got != tt.want {
				t.Errorf("StorageSpec.IsJoinable() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestKinePartialConfigLoading(t *testing.T) {
	yaml := `
spec:
  storage:
    type: kine
`
	c, err := ConfigFromString(yaml)
	assert.NoError(t, err)
	assert.Equal(t, "kine", c.Spec.Storage.Type)
	assert.NotNil(t, c.Spec.Storage.Kine)
	assert.Equal(t, "sqlite:///var/lib/k0s/db/state.db?mode=rwc&_journal=WAL&cache=shared", c.Spec.Storage.Kine.DataSource)
}

type storageSuite struct {
	suite.Suite
}

func (s *storageSuite) TestValidation() {
	var validStorageSpecs = []struct {
		desc string
		spec *StorageSpec
	}{
		{
			desc: "default_storage_spec_is_valid",
			spec: DefaultStorageSpec(),
		},
		{
			desc: "internal_cluster_spec_is_valid",
			spec: &StorageSpec{
				Type: EtcdStorageType,
				Etcd: &EtcdConfig{
					PeerAddress: "192.168.10.10",
				},
			},
		},
		{
			desc: "external_cluster_spec_without_tls_is_valid",
			spec: &StorageSpec{
				Type: EtcdStorageType,
				Etcd: &EtcdConfig{
					ExternalCluster: &ExternalCluster{
						Endpoints:  []string{"http://192.168.10.10"},
						EtcdPrefix: "tenant-1",
					},
				},
			},
		},
		{
			desc: "external_cluster_spec_with_tls_is_valid",
			spec: &StorageSpec{
				Type: EtcdStorageType,
				Etcd: &EtcdConfig{
					ExternalCluster: &ExternalCluster{
						Endpoints:      []string{"http://192.168.10.10"},
						EtcdPrefix:     "tenant-1",
						CaFile:         "/etc/pki/CA/ca.crt",
						ClientCertFile: "/etc/pki/tls/certs/etcd-client.crt",
						ClientKeyFile:  "/etc/pki/tls/private/etcd-client.key",
					},
				},
			},
		},
	}

	for _, tt := range validStorageSpecs {
		s.T().Run(tt.desc, func(t *testing.T) {
			s.Nil(tt.spec.Validate())
		})
	}

	var singleValidationErrorCases = []struct {
		desc           string
		spec           *StorageSpec
		expectedErrMsg string
	}{
		{
			desc: "external_cluster_endpoints_cannot_be_null",
			spec: &StorageSpec{
				Type: EtcdStorageType,
				Etcd: &EtcdConfig{
					ExternalCluster: &ExternalCluster{
						Endpoints:  nil,
						EtcdPrefix: "tenant-1",
					},
				},
			},
			expectedErrMsg: "spec.storage.etcd.externalCluster.endpoints cannot be null or empty",
		},
		{
			desc: "external_cluster_endpoints_cannot_contain_empty_strings",
			spec: &StorageSpec{
				Type: EtcdStorageType,
				Etcd: &EtcdConfig{
					ExternalCluster: &ExternalCluster{
						Endpoints:  []string{"http://192.168.10.2:2379", ""},
						EtcdPrefix: "tenant-1",
					},
				},
			},
			expectedErrMsg: "spec.storage.etcd.externalCluster.endpoints cannot contain empty strings",
		},
		{
			desc: "external_cluster_must_have_configured_all_tls_properties_or_none_of_them",
			spec: &StorageSpec{
				Type: EtcdStorageType,
				Etcd: &EtcdConfig{
					ExternalCluster: &ExternalCluster{
						Endpoints:      []string{"http://192.168.10.10"},
						EtcdPrefix:     "tenant-1",
						CaFile:         "",
						ClientCertFile: "/etc/pki/tls/certs/etcd-client.crt",
						ClientKeyFile:  "",
					},
				},
			},
			expectedErrMsg: "spec.storage.etcd.externalCluster is invalid: all TLS properties [caFile,clientCertFile,clientKeyFile] must be defined or none of those",
		},
	}

	for _, tt := range singleValidationErrorCases {
		s.T().Run(tt.desc, func(t *testing.T) {
			errs := tt.spec.Validate()
			s.NotNil(errs)
			s.Len(errs, 1)
			s.Contains(errs[0].Error(), tt.expectedErrMsg)
		})
	}

	s.T().Run("external_cluster_endpoints_and_etcd_prefix_cannot_be_empty", func(t *testing.T) {
		spec := &StorageSpec{
			Type: EtcdStorageType,
			Etcd: &EtcdConfig{
				ExternalCluster: &ExternalCluster{
					Endpoints:  []string{},
					EtcdPrefix: "",
				},
			},
		}

		errs := spec.Validate()
		s.NotNil(errs)
		s.Len(errs, 2)
		s.Contains(errs[0].Error(), "spec.storage.etcd.externalCluster.endpoints cannot be null or empty")
		s.Contains(errs[1].Error(), "spec.storage.etcd.externalCluster.etcdPrefix cannot be empty")
	})
}

func (s *storageSuite) TestIsTLSEnabled() {
	var storageSpecs = []struct {
		desc           string
		spec           *StorageSpec
		expectedResult bool
	}{
		{
			desc: "is_TLS_enabled_returns_true_when_internal_cluster_is_used",
			spec: &StorageSpec{
				Type: EtcdStorageType,
				Etcd: &EtcdConfig{
					PeerAddress: "192.168.10.10",
				},
			},
			expectedResult: true,
		},
		{
			desc: "is_TLS_enabled_returns_true_when_external_cluster_is_used_and_has_set_all_TLS_properties",
			spec: &StorageSpec{
				Type: EtcdStorageType,
				Etcd: &EtcdConfig{
					ExternalCluster: &ExternalCluster{
						Endpoints:      []string{"http://192.168.10.10"},
						EtcdPrefix:     "tenant-1",
						CaFile:         "/etc/pki/CA/ca.crt",
						ClientCertFile: "/etc/pki/tls/certs/etcd-client.crt",
						ClientKeyFile:  "/etc/pki/tls/private/etcd-client.key",
					},
				},
			},
			expectedResult: true,
		},
		{
			desc: "is_TLS_enabled_returns_false_when_external_cluster_is_used_but_has_no_TLS_properties",
			spec: &StorageSpec{
				Type: EtcdStorageType,
				Etcd: &EtcdConfig{
					ExternalCluster: &ExternalCluster{
						Endpoints:  []string{"http://192.168.10.10"},
						EtcdPrefix: "tenant-1",
					},
				},
			},
			expectedResult: false,
		},
		{
			desc: "is_TLS_enabled_returns_false_when_external_cluster_is_used_but_TLS_properties_are_configured_partially",
			spec: &StorageSpec{
				Type: EtcdStorageType,
				Etcd: &EtcdConfig{
					ExternalCluster: &ExternalCluster{
						Endpoints:  []string{"http://192.168.10.10"},
						EtcdPrefix: "tenant-1",
						CaFile:     "/etc/pki/CA/ca.crt",
					},
				},
			},
			expectedResult: false,
		},
	}

	for _, tt := range storageSpecs {
		s.T().Run(tt.desc, func(t *testing.T) {
			result := tt.spec.Etcd.IsTLSEnabled()
			s.Equal(result, tt.expectedResult)
		})
	}
}

func TestStorageSuite(t *testing.T) {
	storageSuite := &storageSuite{}

	suite.Run(t, storageSuite)
}
