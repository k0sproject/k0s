package v1beta1

type StorageSpec struct {
	Type string      `yaml:"type" validate:"oneof=kine"`
	Kine *KineConfig `yaml:"kine"`
}

type KineConfig struct {
	DataSource string `yaml:"dataSource"`
}

func DefaultStorageSpec() *StorageSpec {
	return &StorageSpec{
		Type: "kine",
		Kine: &KineConfig{
			DataSource: "/var/lib/mke/db/state.db?more=rwc&_journal=WAL&cache=shared",
		},
	}
}
