package v1beta1

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
	Storage *StorageSpec
}

// UnmarshalYAML sets in some sane defaults when unmarshaling the data from yaml
func (c *ClusterConfig) UnmarshalYAML(unmarshal func(interface{}) error) error {
	c.Metadata = &ClusterMeta{
		Name: "mke",
	}
	c.Spec = &ClusterSpec{}

	type yclusterconfig ClusterConfig
	yc := (*yclusterconfig)(c)

	if err := unmarshal(yc); err != nil {
		return err
	}

	return nil
}

func DefaultClusterSpec() ClusterSpec {
	return ClusterSpec{
		Storage: DefaultStorageSpec(),
	}
}
