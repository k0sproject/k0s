package v1beta1

import (
	"testing"

	"github.com/Mirantis/mke/pkg/util"
	"github.com/stretchr/testify/assert"
	yaml "gopkg.in/yaml.v2"
)

func TestClusterDefaults(t *testing.T) {
	c, err := fromYaml(t, "apiVersion: mke.mirantis.com/v1beta1")
	assert.NoError(t, err)
	assert.NotNil(t, c.Metadata)
	assert.Equal(t, "mke", c.Metadata.Name)
	assert.Equal(t, DefaultStorageSpec(), c.Spec.Storage)
}

func TestStorageDefaults(t *testing.T) {
	yamlData := `
apiVersion: mke.mirantis.com/v1beta1
kind: Cluster
metadata:
  name: foobar
`

	c, err := fromYaml(t, yamlData)
	assert.NoError(t, err)
	assert.Equal(t, "kine", c.Spec.Storage.Type)
	assert.Equal(t, DefaultKineDataSource, c.Spec.Storage.Kine.DataSource)
}

func TestEtcdDefaults(t *testing.T) {
	yamlData := `
apiVersion: mke.mirantis.com/v1beta1
kind: Cluster
metadata:
  name: foobar
spec:
  storage:
    type: etcd
`

	c, err := fromYaml(t, yamlData)
	assert.NoError(t, err)
	assert.Equal(t, "etcd", c.Spec.Storage.Type)
	addr, err := util.FirstPublicAddress()
	assert.NoError(t, err)
	assert.Equal(t, addr, c.Spec.Storage.Etcd.PeerAddress)
}

func fromYaml(t *testing.T, yamlData string) (*ClusterConfig, error) {
	config := &ClusterConfig{}
	err := yaml.Unmarshal([]byte(yamlData), &config)
	if err != nil {
		return nil, err
	}

	return config, nil
}
