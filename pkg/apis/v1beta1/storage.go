package v1beta1

import (
	"strings"

	"github.com/Mirantis/mke/pkg/util"
	"github.com/sirupsen/logrus"
)

const (
	EtcdStorageType = "etcd"
	KineStorageType = "kine"
)

// StorageSpec defines the storage related config options
type StorageSpec struct {
	Type string      `yaml:"type"`
	Kine *KineConfig `yaml:"kine"`
	Etcd *EtcdConfig `yaml:"etcd"`
}

// KineConfig defines the Kine related config options
type KineConfig struct {
	DataSource string `yaml:"dataSource"`
}

// DefaultKineDataSource sets the default kine datasource URL
const DefaultKineDataSource = "sqlite:///var/lib/mke/db/state.db?more=rwc&_journal=WAL&cache=shared"

// DefaultStorageSpec creates StorageSpec with sane defaults
func DefaultStorageSpec() *StorageSpec {
	return &StorageSpec{
		Type: EtcdStorageType,
		Etcd: DefaultEtcdConfig(),
	}
}

// IsJoinable returns true only if the storage config is such that another controller can join the cluster
func (s *StorageSpec) IsJoinable() bool {
	if s.Type == EtcdStorageType {
		return true
	}

	if strings.HasPrefix(s.Kine.DataSource, "sqlite://") {
		return false
	}

	if strings.HasPrefix(s.Kine.DataSource, "mysql://") {
		return true
	}

	if strings.HasPrefix(s.Kine.DataSource, "postgres://") {
		return true
	}

	return false
}

// UnmarshalYAML sets in some sane defaults when unmarshaling the data from yaml
func (s *StorageSpec) UnmarshalYAML(unmarshal func(interface{}) error) error {
	s.Type = EtcdStorageType
	s.Etcd = DefaultEtcdConfig()

	type ystorageconfig StorageSpec
	yc := (*ystorageconfig)(s)

	if err := unmarshal(yc); err != nil {
		return err
	}

	if s.Type == KineStorageType && s.Kine == nil {
		s.Kine = DefaultKineConfig()
	}

	return nil
}

// EtcdConfig defines etcd related config options
type EtcdConfig struct {
	PeerAddress string `yaml:"peerAddress"`
}

// DefaultEtcdConfig creates EtcdConfig with sane defaults
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

// DefaultKineConfig creates KineConfig with sane defaults
func DefaultKineConfig() *KineConfig {
	return &KineConfig{
		DataSource: DefaultKineDataSource,
	}
}
