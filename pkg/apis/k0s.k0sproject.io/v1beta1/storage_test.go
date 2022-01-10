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
