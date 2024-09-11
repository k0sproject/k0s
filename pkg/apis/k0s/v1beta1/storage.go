/*
Copyright 2020 k0s authors

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
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/k0sproject/k0s/internal/pkg/iface"
	"github.com/k0sproject/k0s/pkg/config/kine"
	"github.com/k0sproject/k0s/pkg/constant"

	"k8s.io/apimachinery/pkg/util/validation/field"

	"github.com/sirupsen/logrus"
)

var _ Validateable = (*StorageSpec)(nil)

// StorageSpec defines the storage related config options
type StorageSpec struct {
	Etcd *EtcdConfig `json:"etcd,omitempty"`
	Kine *KineConfig `json:"kine,omitempty"`

	// Type of the data store (valid values:etcd or kine)
	// +kubebuilder:default="etcd"
	Type StorageType `json:"type,omitempty"`
}

// StorageType describes which type of bacing storage should be used for the
// Kubernetes API server. The default is [NllbTypeEnvoyProxy].
// +kubebuilder:validation:Enum=etcd;kine
type StorageType string

// supported storage types
const (
	EtcdStorageType StorageType = "etcd"
	KineStorageType StorageType = "kine"
)

// KineConfig defines the Kine related config options
type KineConfig struct {
	// kine datasource URL
	DataSource string `json:"dataSource,omitempty"`
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
	switch s.Type {
	case EtcdStorageType:
		return !s.Etcd.IsExternalClusterUsed()
	case KineStorageType:
		return s.Kine.IsJoinable()
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
	if s == nil {
		return nil
	}

	var errors []error

	if s.Type == "" {
		errors = append(errors, field.Required(field.NewPath("type"), ""))
	} else if types := []StorageType{EtcdStorageType, KineStorageType}; !slices.Contains(types, s.Type) {
		errors = append(errors, field.NotSupported(field.NewPath("type"), s.Type, types))
	}

	if s.Etcd != nil && s.Etcd.ExternalCluster != nil {
		errors = append(errors, validateRequiredProperties(s.Etcd.ExternalCluster)...)
		errors = append(errors, validateOptionalTLSProperties(s.Etcd.ExternalCluster)...)
	}

	return errors
}

// EtcdConfig defines etcd related config options
type EtcdConfig struct {
	// ExternalCluster defines external etcd cluster related config options
	ExternalCluster *ExternalCluster `json:"externalCluster,omitempty"`

	// Node address used for etcd cluster peering
	PeerAddress string `json:"peerAddress,omitempty"`

	// Map of key-values (strings) for any extra arguments you want to pass down to the etcd process
	ExtraArgs map[string]string `json:"extraArgs,omitempty"`
}

// ExternalCluster defines external etcd cluster related config options
type ExternalCluster struct {
	// Endpoints of external etcd cluster used to connect by k0s
	// +kubebuilder:validation:MinItems=1
	Endpoints []string `json:"endpoints"`

	// EtcdPrefix is a prefix to prepend to all resource paths in etcd
	EtcdPrefix string `json:"etcdPrefix,omitempty"`

	// CaFile is the host path to a file with CA certificate
	CaFile string `json:"caFile,omitempty"`

	// ClientCertFile is the host path to a file with TLS certificate for etcd client
	ClientCertFile string `json:"clientCertFile,omitempty"`

	// ClientKeyFile is the host path to a file with TLS key for etcd client
	ClientKeyFile string `json:"clientKeyFile,omitempty"`
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
		ExtraArgs:       make(map[string]string),
	}
}

const etcdNameExtraArg = "name"

// GetNodeName returns the node name for the etcd peer
func (e *EtcdConfig) GetNodeName() (string, error) {
	if e.ExtraArgs != nil && e.ExtraArgs[etcdNameExtraArg] != "" {
		return e.ExtraArgs[etcdNameExtraArg], nil
	}

	return os.Hostname()
}

// DefaultKineConfig creates KineConfig with sane defaults
func DefaultKineConfig(dataDir string) *KineConfig {
	return &KineConfig{
		// https://www.sqlite.org/c3ref/open.html#urifilenamesinsqlite3open
		DataSource: fmt.Sprintf("sqlite://%s", &url.URL{
			Scheme:   "file",
			OmitHost: true,
			Path:     filepath.ToSlash(filepath.Join(dataDir, "db", "state.db")),
			RawQuery: "mode=rwc&_journal=WAL",
		}),
	}
}

func (k *KineConfig) IsJoinable() bool {
	backend, dsn, err := kine.SplitDataSource(k.DataSource)
	if err != nil {
		return false
	}

	switch backend {
	case "sqlite":
		return false

	case "nats":
		if u, err := url.Parse(dsn); err == nil {
			if q, err := url.ParseQuery(u.RawQuery); err == nil {
				return q.Has("noEmbed")
			}
		}
		return false
	}

	return true
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

// IsExternalClusterUsed returns true if `spec.storage.etcd.externalCluster` is defined, otherwise returns false.
func (e *EtcdConfig) IsExternalClusterUsed() bool {
	return e != nil && e.ExternalCluster != nil
}

// IsTLSEnabled returns true if external cluster is not configured or external cluster is configured
// with all TLS properties: caFile, clientCertFile, clientKeyFile. Otherwise it returns false.
func (e *EtcdConfig) IsTLSEnabled() bool {
	return !e.IsExternalClusterUsed() || e.ExternalCluster.hasAllTLSPropertiesDefined()
}

// GetCaFilePath returns the host path to a file with CA certificate if external cluster has configured all TLS properties,
// otherwise it returns the host path to a default CA certificate in a given certDir directory.
func (e *EtcdConfig) GetCaFilePath(certDir string) string {
	if e.IsExternalClusterUsed() && e.ExternalCluster.hasAllTLSPropertiesDefined() {
		return e.ExternalCluster.CaFile
	}
	return filepath.Join(certDir, "ca.crt")
}

// GetCertFilePath returns the host path to a file with a client certificate if external cluster has configured all TLS properties,
// otherwise it returns the host path to a default client certificate in a given certDir directory.
func (e *EtcdConfig) GetCertFilePath(certDir string) string {
	if e.IsExternalClusterUsed() && e.ExternalCluster.hasAllTLSPropertiesDefined() {
		return e.ExternalCluster.ClientCertFile
	}
	return filepath.Join(certDir, "apiserver-etcd-client.crt")
}

// GetCaFilePath returns the host path to a file with client private key if external cluster has configured all TLS properties,
// otherwise it returns the host path to a default client private key in a given certDir directory.
func (e *EtcdConfig) GetKeyFilePath(certDir string) string {
	if e.IsExternalClusterUsed() && e.ExternalCluster.hasAllTLSPropertiesDefined() {
		return e.ExternalCluster.ClientKeyFile
	}
	return filepath.Join(certDir, "apiserver-etcd-client.key")
}

func validateRequiredProperties(e *ExternalCluster) []error {
	var errors []error

	if len(e.Endpoints) == 0 {
		errors = append(errors, fmt.Errorf("spec.storage.etcd.externalCluster.endpoints cannot be null or empty"))
	} else if slices.Contains(e.Endpoints, "") {
		errors = append(errors, fmt.Errorf("spec.storage.etcd.externalCluster.endpoints cannot contain empty strings"))
	}

	if e.EtcdPrefix == "" {
		errors = append(errors, fmt.Errorf("spec.storage.etcd.externalCluster.etcdPrefix cannot be empty"))
	}

	return errors
}

func validateOptionalTLSProperties(e *ExternalCluster) []error {
	noTLSPropertyDefined := e.CaFile == "" && e.ClientCertFile == "" && e.ClientKeyFile == ""

	if noTLSPropertyDefined || e.hasAllTLSPropertiesDefined() {
		return nil
	}
	return []error{fmt.Errorf("spec.storage.etcd.externalCluster is invalid: " +
		"all TLS properties [caFile,clientCertFile,clientKeyFile] must be defined or none of those")}
}

func (e *ExternalCluster) hasAllTLSPropertiesDefined() bool {
	return e.CaFile != "" && e.ClientCertFile != "" && e.ClientKeyFile != ""
}
