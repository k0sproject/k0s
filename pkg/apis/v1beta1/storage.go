/*
Copyright 2020 Mirantis, Inc.

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
	"strings"

	"github.com/sirupsen/logrus"

	"github.com/k0sproject/k0s/internal/util"
)

// supported storage types
const (
	EtcdStorageType = "etcd"
	KineStorageType = "kine"
)

// StorageSpec defines the storage related config options
type StorageSpec struct {
	Type string      `yaml:"type"`
	Kine *KineConfig `yaml:"kine,omitempty"`
	Etcd *EtcdConfig `yaml:"etcd"`
}

// KineConfig defines the Kine related config options
type KineConfig struct {
	DataSource string `yaml:"dataSource"`
}

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
	return nil
}

// Validate validates storage specs correctness
func (s *StorageSpec) Validate() []error {
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
func DefaultKineConfig(dataDir string) *KineConfig {
	return &KineConfig{
		DataSource: "sqlite://" + dataDir + "/db/state.db?more=rwc&_journal=WAL&cache=shared",
	}
}
