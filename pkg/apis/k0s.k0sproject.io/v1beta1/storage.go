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
	"encoding/json"
	"fmt"
	"k8s.io/utils/strings/slices"
	"path/filepath"
	"strings"

	"github.com/sirupsen/logrus"

	"github.com/k0sproject/k0s/internal/pkg/iface"
	"github.com/k0sproject/k0s/pkg/constant"
)

// supported storage types
const (
	EtcdStorageType = "etcd"
	KineStorageType = "kine"
)

var _ Validateable = (*StorageSpec)(nil)

// StorageSpec defines the storage related config options
type StorageSpec struct {
	Etcd *EtcdConfig `json:"etcd"`
	Kine *KineConfig `json:"kine,omitempty"`

	// Type of the data store (valid values:etcd or kine)
	Type string `json:"type"`
}

// KineConfig defines the Kine related config options
type KineConfig struct {
	// kine datasource URL
	DataSource string `json:"dataSource"`
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
		return !s.Etcd.IsExternalClusterUsed()
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

// UnmarshalJSON sets in some sane defaults when unmarshaling the data from json
func (s *StorageSpec) UnmarshalJSON(data []byte) error {
	s.Type = EtcdStorageType
	s.Etcd = DefaultEtcdConfig()

	type storage StorageSpec
	jc := (*storage)(s)

	if err := json.Unmarshal(data, jc); err != nil {
		return err
	}

	if jc.Type == KineStorageType && jc.Kine == nil {
		jc.Kine = DefaultKineConfig(constant.DataDirDefault)
	}
	return nil
}

// Validate validates storage specs correctness
func (s *StorageSpec) Validate() []error {
	var errors []error

	if s.Etcd != nil && s.Etcd.ExternalCluster != nil {
		if s.Etcd.ExternalCluster.Endpoints == nil || len(s.Etcd.ExternalCluster.Endpoints) == 0 {
			errors = append(errors, fmt.Errorf("spec.storage.etcd.externalCluster.endpoints cannot be null or empty"))
		} else if slices.Contains(s.Etcd.ExternalCluster.Endpoints, "") {
			errors = append(errors, fmt.Errorf("spec.storage.etcd.externalCluster.endpoints cannot contain empty strings"))
		}
		if s.Etcd.ExternalCluster.EtcdPrefix == "" {
			errors = append(errors, fmt.Errorf("spec.storage.etcd.externalCluster.etcdPrefix cannot be empty"))
		}
	}

	return errors
}

// EtcdConfig defines etcd related config options
type EtcdConfig struct {
	// ExternalCluster defines external etcd cluster related config options
	ExternalCluster *ExternalCluster `json:"externalCluster"`

	// Node address used for etcd cluster peering
	PeerAddress string `json:"peerAddress"`
}

// ExternalCluster defines external etcd cluster related config options
type ExternalCluster struct {
	// Endpoints of external etcd cluster used to connect by k0s
	Endpoints []string `json:"endpoints"`

	// EtcdPrefix is a prefix to prepend to all resource paths in etcd
	EtcdPrefix string `json:"etcdPrefix"`
}

// DefaultEtcdConfig creates EtcdConfig with sane defaults
func DefaultEtcdConfig() *EtcdConfig {
	addr, err := iface.FirstPublicAddress()
	if err != nil {
		logrus.Warnf("failed to resolve etcd peering address automatically, using loopback")
		addr = "127.0.0.1"
	}
	return &EtcdConfig{
		ExternalCluster: nil,
		PeerAddress:     addr,
	}
}

// DefaultKineConfig creates KineConfig with sane defaults
func DefaultKineConfig(dataDir string) *KineConfig {
	return &KineConfig{
		DataSource: "sqlite://" + dataDir + "/db/state.db?more=rwc&_journal=WAL&cache=shared",
	}
}

// GetEndpointsAsString returns comma-separated list of external cluster endpoints if exist
// or internal etcd address which is https://127.0.0.1:2379
func (e *EtcdConfig) GetEndpointsAsString() string {
	if e != nil && e.IsExternalClusterUsed() {
		return strings.Join(e.ExternalCluster.Endpoints, ",")
	}
	return "https://127.0.0.1:2379"
}

// GetEndpointsAsString returns external cluster endpoints if exist
// or internal etcd address which is https://127.0.0.1:2379
func (e *EtcdConfig) GetEndpoints() []string {
	if e != nil && e.IsExternalClusterUsed() {
		return e.ExternalCluster.Endpoints
	}
	return []string{"https://127.0.0.1:2379"}
}

func (e *EtcdConfig) IsExternalClusterUsed() bool {
	return e != nil && e.ExternalCluster != nil
}

func (e *EtcdConfig) IsTLSEnabled() bool {
	return !e.IsExternalClusterUsed()
}

func (e *EtcdConfig) GetCertFilePath(certDir string) string {
	return filepath.Join(certDir, "apiserver-etcd-client.crt")
}

func (e *EtcdConfig) GetKeyFilePath(certDir string) string {
	return filepath.Join(certDir, "apiserver-etcd-client.key")
}

func (e *EtcdConfig) GetCaFilePath(certDir string) string {
	return filepath.Join(certDir, "ca.crt")
}
