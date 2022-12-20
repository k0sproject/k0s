/*
Copyright 2022 k0s authors

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

package config

import (
	"testing"

	"k8s.io/apimachinery/pkg/api/resource"
	logsv1 "k8s.io/component-base/logs/api/v1"
	kubeletv1beta1 "k8s.io/kubelet/config/v1beta1"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestToConfigMapData(t *testing.T) {
	for _, test := range append(roundtripTests,
		roundtripTest{"nil_produces_zero", nil, map[string]string{}},
	) {
		t.Run(test.name, func(t *testing.T) {
			data, err := ToConfigMapData(test.profile)
			require.NoError(t, err)
			assert.Equal(t, test.data, data)
		})
	}
}

func TestFromConfigMapData(t *testing.T) {
	for _, test := range append(roundtripTests,
		roundtripTest{"nil_produces_zero", &Profile{}, nil},
	) {
		t.Run(test.name, func(t *testing.T) {
			config, err := FromConfigMapData(test.data)
			require.NoError(t, err)
			require.NotNil(t, config)
			assert.Equal(t, test.profile, config)
		})
	}

	t.Run("collects_errors", func(t *testing.T) {
		data := map[string]string{
			"kubeletConfiguration": "1",
		}

		config, err := FromConfigMapData(data)
		assert.ErrorContains(t, err, "json: cannot unmarshal number into Go value of type")
		assert.Nil(t, config)
	})
}

type roundtripTest struct {
	name    string
	profile *Profile
	data    map[string]string
}

var roundtripTests = []roundtripTest{
	{"empty", &Profile{}, map[string]string{}},
	{
		"kubelet",
		&Profile{
			KubeletConfiguration: kubeletv1beta1.KubeletConfiguration{
				Logging: logsv1.LoggingConfiguration{
					Options: logsv1.FormatOptions{
						JSON: logsv1.JSONOptions{
							InfoBufferSize: resource.QuantityValue{
								// This will be set as default by the unmarshaler.
								Quantity: resource.MustParse("0"),
							},
						},
					},
				},
			},
		},
		map[string]string{
			"kubeletConfiguration": `{"syncFrequency":"0s","fileCheckFrequency":"0s","httpCheckFrequency":"0s","authentication":{"x509":{},"webhook":{"cacheTTL":"0s"},"anonymous":{}},"authorization":{"webhook":{"cacheAuthorizedTTL":"0s","cacheUnauthorizedTTL":"0s"}},"streamingConnectionIdleTimeout":"0s","nodeStatusUpdateFrequency":"0s","nodeStatusReportFrequency":"0s","imageMinimumGCAge":"0s","volumeStatsAggPeriod":"0s","cpuManagerReconcilePeriod":"0s","runtimeRequestTimeout":"0s","evictionPressureTransitionPeriod":"0s","memorySwap":{},"logging":{"flushFrequency":0,"verbosity":0,"options":{"json":{"infoBufferSize":"0"}}},"shutdownGracePeriod":"0s","shutdownGracePeriodCriticalPods":"0s"}`,
		},
	},
}
