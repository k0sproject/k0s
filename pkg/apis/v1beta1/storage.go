package v1beta1

type StorageSpec struct {
	Type string      `yaml:"type"`
	Kine *KineConfig `yaml:"kine"`
	Etcd *EtcdConfig `yaml:"etcd"`
}

type KineConfig struct {
	DataSource string `yaml:"dataSource"`
}

func DefaultStorageSpec() *StorageSpec {
	return &StorageSpec{
		Type: "kine",
		Kine: &KineConfig{
			DataSource: "sqlite:///var/lib/mke/db/state.db?more=rwc&_journal=WAL&cache=shared",
		},
	}
}

type EtcdConfig struct {
	PeerAddress string `yaml:"peerAddress"`
}
