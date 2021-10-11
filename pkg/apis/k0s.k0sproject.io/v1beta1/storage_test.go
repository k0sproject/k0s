/*
Copyright 2021 k0s authors

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
	assert.Equal(t, "sqlite:///var/lib/k0s/db/state.db?more=rwc&_journal=WAL&cache=shared", c.Spec.Storage.Kine.DataSource)
}

type storageSuite struct {
	suite.Suite
}

func (s *storageSuite) TestValidation() {
	s.T().Run("default_storage_spec_is_valid", func(t *testing.T) {
		spec := DefaultStorageSpec(constant.DataDirDefault)

		s.Nil(spec.Validate())
	})

	s.T().Run("external_cluster_spec_is_valid", func(t *testing.T) {
		spec := &StorageSpec{
			Type: EtcdStorageType,
			Etcd: &EtcdConfig{
				ExternalCluster: &ExternalCluster{
					Endpoints:  []string{"http://192.168.10.10"},
					EtcdPrefix: "tenant-1",
				},
				PeerAddress: "",
			},
		}

		s.Nil(spec.Validate())
	})

	s.T().Run("external_cluster_endpoints_and_etcd_prefix_cannot_be_empty", func(t *testing.T) {
		spec := &StorageSpec{
			Type: EtcdStorageType,
			Etcd: &EtcdConfig{
				ExternalCluster: &ExternalCluster{
					Endpoints:  []string{},
					EtcdPrefix: "",
				},
				PeerAddress: "",
			},
		}

		errs := spec.Validate()
		s.NotNil(errs)
		s.Len(errs, 2)
		s.Contains(errs[0].Error(), "spec.storage.etcd.externalCluster.endpoints cannot be null or empty")
		s.Contains(errs[1].Error(), "spec.storage.etcd.externalCluster.etcdPrefix cannot be empty")
	})

	s.T().Run("external_cluster_endpoints_cannot_be_null", func(t *testing.T) {
		spec := &StorageSpec{
			Type: EtcdStorageType,
			Etcd: &EtcdConfig{
				ExternalCluster: &ExternalCluster{
					Endpoints:  nil,
					EtcdPrefix: "tenant-1",
				},
				PeerAddress: "",
			},
		}

		errs := spec.Validate()
		s.NotNil(errs)
		s.Len(errs, 1)
		s.Contains(errs[0].Error(), "spec.storage.etcd.externalCluster.endpoints cannot be null or empty")
	})

	s.T().Run("external_cluster_endpoints_cannot_contain_empty_strings", func(t *testing.T) {
		spec := &StorageSpec{
			Type: EtcdStorageType,
			Etcd: &EtcdConfig{
				ExternalCluster: &ExternalCluster{
					Endpoints:  []string{"http://192.168.10.2:2379", ""},
					EtcdPrefix: "tenant-1",
				},
				PeerAddress: "",
			},
		}

		errs := spec.Validate()
		s.NotNil(errs)
		s.Len(errs, 1)
		s.Contains(errs[0].Error(), "spec.storage.etcd.externalCluster.endpoints cannot contain empty strings")
	})
}

func TestStorageSuite(t *testing.T) {
	storageSuite := &storageSuite{}

	suite.Run(t, storageSuite)
}
