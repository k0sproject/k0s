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
	"testing"

	"github.com/stretchr/testify/assert"
	"gopkg.in/yaml.v2"

	"github.com/k0sproject/k0s/internal/util"
)

func TestClusterDefaults(t *testing.T) {
	c, err := fromYaml(t, "apiVersion: k0s.k0sproject.io/v1beta1")
	assert.NoError(t, err)
	assert.NotNil(t, c.Metadata)
	assert.Equal(t, "k0s", c.Metadata.Name)
	assert.Equal(t, DefaultStorageSpec(), c.Spec.Storage)
}

func TestStorageDefaults(t *testing.T) {
	yamlData := `
apiVersion: k0s.k0sproject.io/v1beta1
kind: Cluster
metadata:
  name: foobar
`

	c, err := fromYaml(t, yamlData)
	assert.NoError(t, err)
	assert.Equal(t, "etcd", c.Spec.Storage.Type)
	addr, err := util.FirstPublicAddress()
	assert.NoError(t, err)
	assert.Equal(t, addr, c.Spec.Storage.Etcd.PeerAddress)
}

func TestEtcdDefaults(t *testing.T) {
	yamlData := `
apiVersion: k0s.k0sproject.io/v1beta1
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

func TestNetworkValidation_Custom(t *testing.T) {
	yamlData := `
apiVersion: k0s.k0sproject.io/v1beta1
kind: Cluster
metadata:
  name: foobar
spec:
  network:
    provider: custom
  storage:
    type: etcd
`

	c, err := fromYaml(t, yamlData)
	assert.NoError(t, err)
	errors := c.Validate()
	assert.Equal(t, 0, len(errors))
}

func TestNetworkValidation_Calico(t *testing.T) {
	yamlData := `
apiVersion: k0s.k0sproject.io/v1beta1
kind: Cluster
metadata:
  name: foobar
spec:
  network:
    provider: calico
  storage:
    type: etcd
`

	c, err := fromYaml(t, yamlData)
	assert.NoError(t, err)
	errors := c.Validate()
	assert.Equal(t, 0, len(errors))
}

func TestNetworkValidation_Invalid(t *testing.T) {
	yamlData := `
apiVersion: k0s.k0sproject.io/v1beta1
kind: Cluster
metadata:
  name: foobar
spec:
  network:
    provider: invalidProvider
  storage:
    type: etcd
`

	c, err := fromYaml(t, yamlData)
	assert.NoError(t, err)
	errors := c.Validate()
	assert.Equal(t, 1, len(errors))
	assert.Equal(t, "unsupported network provider: invalidProvider", errors[0].Error())
}
