package v1beta1

import (
	"testing"
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
