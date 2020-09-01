package v1beta1

import (
	"fmt"
	"io/ioutil"

	"github.com/Mirantis/mke/pkg/util"
	"github.com/pkg/errors"
	yaml "gopkg.in/yaml.v2"
)

// ClusterConfig ...
type ClusterConfig struct {
	APIVersion string       `yaml:"apiVersion" validate:"eq=mke.mirantis.com/v1beta1"`
	Kind       string       `yaml:"kind" validate:"eq=Cluster"`
	Metadata   *ClusterMeta `yaml:"metadata"`
	Spec       *ClusterSpec `yaml:"spec"`
}

// ClusterMeta ...
type ClusterMeta struct {
	Name string `yaml:"name" validate:"required"`
}

// ClusterSpec ...
type ClusterSpec struct {
	API               *APISpec               `yaml:"api"`
	ControllerManager *ControllerManagerSpec `yaml:"controllerManager"`
	Scheduler         *SchedulerSpec         `yaml:"scheduler"`
	Storage           *StorageSpec           `yaml:"storage"`
	Network           *Network               `yaml:"network"`
}

// APISpec ...
type APISpec struct {
	Address   string            `yaml:"address"`
	SANs      []string          `yaml:"sans"`
	ExtraArgs map[string]string `yaml:"extraArgs"`
}

// ControllerManagerSpec ...
type ControllerManagerSpec struct {
	ExtraArgs map[string]string `yaml:"extraArgs"`
}

// SchedulerSpec ...
type SchedulerSpec struct {
	ExtraArgs map[string]string `yaml:"extraArgs"`
}

// Validate validates cluster config
func (c *ClusterConfig) Validate() []error {
	var errors []error

	errors = append(errors, c.Spec.Network.Validate()...)
	// TODO We need to validate all other parts too

	return errors
}

// APIAddress ...
func (a *APISpec) APIAddress() string {
	return fmt.Sprintf("https://%s:6443", a.Address)
}

// ControllerJoinAddress returns the controller join APIs address
func (a *APISpec) ControllerJoinAddress() string {
	return fmt.Sprintf("https://%s:9443", a.Address)
}

// FromYaml ...
func FromYaml(filename string) (*ClusterConfig, error) {
	buf, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to read config file at %s", filename)
	}

	config := &ClusterConfig{}
	err = yaml.Unmarshal(buf, &config)
	if err != nil {
		return config, err
	}

	if config.Spec == nil {
		config.Spec = DefaultClusterSpec()
	}

	return config, nil
}

// DefaultClusterConfig ...
func DefaultClusterConfig() *ClusterConfig {
	return &ClusterConfig{
		Spec: DefaultClusterSpec(),
	}
}

// UnmarshalYAML sets in some sane defaults when unmarshaling the data from yaml
func (c *ClusterConfig) UnmarshalYAML(unmarshal func(interface{}) error) error {
	c.Kind = "Cluster"
	c.Metadata = &ClusterMeta{
		Name: "mke",
	}
	c.Spec = DefaultClusterSpec()

	type yclusterconfig ClusterConfig
	yc := (*yclusterconfig)(c)

	if err := unmarshal(yc); err != nil {
		return err
	}

	return nil
}

// DefaultAPISpec ...
func DefaultAPISpec() *APISpec {
	// Collect all nodes addresses for sans
	addresses, _ := util.AllAddresses()
	publicAddress, _ := util.FirstPublicAddress()
	return &APISpec{
		SANs:    append(addresses, publicAddress),
		Address: publicAddress,
	}
}

// DefaultClusterSpec ...
func DefaultClusterSpec() *ClusterSpec {
	return &ClusterSpec{
		Storage:           DefaultStorageSpec(),
		Network:           DefaultNetwork(),
		API:               DefaultAPISpec(),
		ControllerManager: &ControllerManagerSpec{},
		Scheduler:         &SchedulerSpec{},
	}
}
