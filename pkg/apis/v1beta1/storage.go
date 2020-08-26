package v1beta1

import (
	"github.com/Mirantis/mke/pkg/util"
	"github.com/sirupsen/logrus"
)

type StorageSpec struct {
	Type string      `yaml:"type"`
	Kine *KineConfig `yaml:"kine"`
	Etcd *EtcdConfig `yaml:"etcd"`
}

type KineConfig struct {
	DataSource string `yaml:"dataSource"`
}

const DefaultKineDataSource = "sqlite:///var/lib/mke/db/state.db?more=rwc&_journal=WAL&cache=shared"

func DefaultStorageSpec() *StorageSpec {
	return &StorageSpec{
		Type: "kine",
		Kine: &KineConfig{
			DataSource: DefaultKineDataSource,
		},
	}
}

// UnmarshalYAML sets in some sane defaults when unmarshaling the data from yaml
func (s *StorageSpec) UnmarshalYAML(unmarshal func(interface{}) error) error {
	s.Type = "kine"
	s.Kine = &KineConfig{
		DataSource: DefaultKineDataSource,
	}

	type ystorageconfig StorageSpec
	yc := (*ystorageconfig)(s)

	if err := unmarshal(yc); err != nil {
		return err
	}

	if s.Type == "etcd" && s.Etcd == nil {
		s.Etcd = DefaultEtcdConfig()
	}

	return nil
}

type EtcdConfig struct {
	PeerAddress string `yaml:"peerAddress"`
}

func DefaultEtcdConfig() *EtcdConfig {
	addr, err := util.FirstPublicAddress()
	if err != nil {
		logrus.Warnf("failed to resolve etcd peering address automatically, using loopback")
		addr = "127.0.0.1"
	}
	return &EtcdConfig{
		PeerAddress: addr,
	}
}
