// SPDX-FileCopyrightText: 2023 k0s authors
// SPDX-License-Identifier: Apache-2.0

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
		m := map[string]any{}
		assert.NoError(t, yaml.Unmarshal(b.Bytes(), &m))
		kubeProxyConfigData := map[string]any{}
		assert.NoError(t, yaml.Unmarshal([]byte(m["data"].(map[string]any)["config.conf"].(string)), &kubeProxyConfigData))
		renderedFeatureGates := kubeProxyConfigData["featureGates"].(map[string]any)
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
