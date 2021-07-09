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
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/k0sproject/k0s/internal/util"
	"github.com/k0sproject/k0s/pkg/constant"
)

var k0sVars = constant.CfgVars{}

func TestClusterDefaults(t *testing.T) {
	c, err := configFromString("apiVersion: k0s.k0sproject.io/v1beta1", k0sVars)
	assert.NoError(t, err)
	assert.Equal(t, DefaultStorageSpec(constant.GetConfig("")), c.Spec.Storage)
}

func TestStorageDefaults(t *testing.T) {
	yamlData := `
apiVersion: k0s.k0sproject.io/v1beta1
kind: Cluster
metadata:
  name: foobar
`

	c, err := configFromString(yamlData, k0sVars)
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

	c, err := configFromString(yamlData, k0sVars)
	assert.NoError(t, err)
	assert.Equal(t, "etcd", c.Spec.Storage.Type)
	addr, err := util.FirstPublicAddress()
	assert.NoError(t, err)
	assert.Equal(t, addr, c.Spec.Storage.Etcd.PeerAddress)
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

	c, err := configFromString(yamlData, k0sVars)
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

	c, err := configFromString(yamlData, k0sVars)
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

	c, err := configFromString(yamlData, k0sVars)
	assert.NoError(t, err)
	errors := c.Validate()
	assert.Equal(t, 1, len(errors))
	assert.Equal(t, "unsupported network provider: invalidProvider", errors[0].Error())
}

func TestApiExternalAddress(t *testing.T) {
	yamlData := `
apiVersion: k0s.k0sproject.io/v1beta1
kind: Cluster
metadata:
  name: foobar
spec:
  api:
    externalAddress: foo.bar.com
    address: 1.2.3.4
`

	c, err := configFromString(yamlData, k0sVars)
	assert.NoError(t, err)
	assert.Equal(t, "https://foo.bar.com:6443", c.Spec.API.APIAddressURL())
	assert.Equal(t, "https://foo.bar.com:9443", c.Spec.API.K0sControlPlaneAPIAddress())
}

func TestApiNoExternalAddress(t *testing.T) {
	yamlData := `
apiVersion: k0s.k0sproject.io/v1beta1
kind: Cluster
metadata:
  name: foobar
spec:
  api:
    address: 1.2.3.4
`

	c, err := configFromString(yamlData, k0sVars)
	assert.NoError(t, err)
	assert.Equal(t, "https://1.2.3.4:6443", c.Spec.API.APIAddressURL())
	assert.Equal(t, "https://1.2.3.4:9443", c.Spec.API.K0sControlPlaneAPIAddress())
}
