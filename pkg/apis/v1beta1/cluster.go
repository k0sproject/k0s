package v1beta1

import (
	"fmt"
	"io/ioutil"

	"github.com/Mirantis/mke/pkg/util"
	"github.com/pkg/errors"
	"gopkg.in/yaml.v2"
)

type ClusterConfig struct {
	APIVersion string       `yaml:"apiVersion" validate:"eq=mke.mirantis.com/v1beta1"`
	Kind       string       `yaml:"kind" validate:"eq=Cluster"`
	Metadata   *ClusterMeta `yaml:"metadata"`
	Spec       *ClusterSpec `yaml:"spec"`
}

type ClusterMeta struct {
	Name string `yaml:"name" validate:"required"`
}

type ClusterSpec struct {
	API     *APISpec     `yaml:"api"`
	Storage *StorageSpec `yaml:"storage"`
	Network *Network     `yaml:"network"`
}

type APISpec struct {
	Address string   `yaml:"address"`
	SANs    []string `yaml:"sans"`
}

func (a *APISpec) APIAddress() string {
	return fmt.Sprintf("https://%s:6443", a.Address)
}

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

func DefaultClusterConfig() *ClusterConfig {
	return &ClusterConfig{
		Spec: DefaultClusterSpec(),
	}
}

// UnmarshalYAML sets in some sane defaults when unmarshaling the data from yaml
func (c *ClusterConfig) UnmarshalYAML(unmarshal func(interface{}) error) error {
	c.Metadata = &ClusterMeta{
		Name: "mke",
	}
	c.Spec = &ClusterSpec{
		Storage: DefaultStorageSpec(),
		Network: DefaultNetwork(),
		API:     DefaultAPISpec(),
	}

	type yclusterconfig ClusterConfig
	yc := (*yclusterconfig)(c)

	if err := unmarshal(yc); err != nil {
		return err
	}

	return nil
}

func DefaultAPISpec() *APISpec {
	// Collect all nodes addresses for sans
	addresses, _ := util.AllAddresses()
	publicAddress, _ := util.FirstPublicAddress()
	return &APISpec{
		SANs:    append(addresses, publicAddress),
		Address: publicAddress,
	}
}

func DefaultClusterSpec() *ClusterSpec {
	return &ClusterSpec{
		Storage: DefaultStorageSpec(),
		Network: DefaultNetwork(),
		API:     DefaultAPISpec(),
	}
}
