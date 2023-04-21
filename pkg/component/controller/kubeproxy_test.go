/*
Copyright 2023 k0s authors

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

package controller

import (
	"bytes"
	"strings"
	"testing"

	"github.com/k0sproject/k0s/internal/pkg/templatewriter"
	"github.com/stretchr/testify/assert"
	"sigs.k8s.io/yaml"
)

func TestKubeProxyConfig(t *testing.T) {

	// helper to get only feature gates related part from the rendered kube-proxy manifest collection
	getFeatureGates := func(cfg proxyConfig) map[string]bool {
		tw := templatewriter.TemplateWriter{
			Name:     "kube-proxy-config",
			Template: strings.Split(proxyTemplate, "---")[4],
			Data:     cfg,
		}
		b := bytes.NewBuffer([]byte{})
		assert.NoError(t, tw.WriteToBuffer(b))
		m := map[string]interface{}{}
		assert.NoError(t, yaml.Unmarshal(b.Bytes(), &m))
		kubeProxyConfigData := map[string]interface{}{}
		assert.NoError(t, yaml.Unmarshal([]byte(m["data"].(map[string]interface{})["config.conf"].(string)), &kubeProxyConfigData))
		renderedFeatureGates := kubeProxyConfigData["featureGates"].(map[string]interface{})
		result := map[string]bool{}
		for k, v := range renderedFeatureGates {
			result[k] = v.(bool)
		}
		return result
	}
	t.Run("feature_gates", func(t *testing.T) {
		config := proxyConfig{
			FeatureGates: map[string]bool{
				"Feature0": true,
				"Feature1": false,
			},
		}
		assert.Equal(t, config.FeatureGates, getFeatureGates(config))
	})

}
