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

	"github.com/k0sproject/k0s/internal/pkg/net"
	"github.com/k0sproject/k0s/pkg/apis/k0s/v1beta1"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	logsv1 "k8s.io/component-base/logs/api/v1"
	kubeletv1beta1 "k8s.io/kubelet/config/v1beta1"
	"k8s.io/utils/ptr"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestToConfigMapData(t *testing.T) {
	for _, test := range roundtripTests {
		t.Run(test.name, func(t *testing.T) {
			data, err := ToConfigMapData(test.profile)
			require.NoError(t, err)
			assert.Equal(t, test.data, data)
		})
	}
}

func TestFromConfigMapData(t *testing.T) {
	for _, test := range roundtripTests {
		t.Run(test.name, func(t *testing.T) {
			config, err := FromConfigMapData(test.data)
			require.NoError(t, err)
			require.NotNil(t, config)
			assert.Equal(t, test.profile, config)
		})
	}

	t.Run("collects_errors", func(t *testing.T) {
		data := map[string]string{
			"apiServerAddresses":     "1",
			"kubeletConfiguration":   "2",
			"nodeLocalLoadBalancing": "3",
			"konnectivity":           "4",
		}

		config, err := FromConfigMapData(data)
		var composite interface{ Unwrap() []error }
		if assert.ErrorAs(t, err, &composite) {
			errs := composite.Unwrap()
			assert.Len(t, errs, 4)
			for i, err := range errs {
				assert.ErrorContains(t, err, "json: cannot unmarshal number into Go value of type", "For error #%d", i+1)
			}
		}
		assert.Nil(t, config)
	})

	t.Run("validation", func(t *testing.T) {
		config, err := FromConfigMapData(map[string]string{
			"nodeLocalLoadBalancing": `{"type": "Bogus"}`,
		})
		assert.ErrorContains(t, err, `nodeLocalLoadBalancing.type: Unsupported value: "Bogus": supported values:`)
		assert.Nil(t, config)
	})
}

type roundtripTest struct {
	name    string
	profile *Profile
	data    map[string]string
}

var roundtripTests = []roundtripTest{
	{"empty",
		&Profile{Konnectivity: Konnectivity{AgentPort: 1337}},
		map[string]string{"konnectivity": `{"agentPort":1337}`}},
	{
		"empty_servers",
		&Profile{
			APIServerAddresses: []net.HostPort{},
			Konnectivity:       Konnectivity{AgentPort: 1337},
		},
		map[string]string{
			"apiServerAddresses": "[]",
			"konnectivity":       `{"agentPort":1337}`,
		},
	},
	{
		"servers",
		&Profile{
			APIServerAddresses: []net.HostPort{
				makeHostPort("127.0.0.1", 6443),
				makeHostPort("127.0.0.2", 7443),
			},
			Konnectivity: Konnectivity{AgentPort: 1337},
		},
		map[string]string{
			"apiServerAddresses": `["127.0.0.1:6443","127.0.0.2:7443"]`,
			"konnectivity":       `{"agentPort":1337}`,
		},
	},
	{
		"kubelet",
		&Profile{
			KubeletConfiguration: kubeletv1beta1.KubeletConfiguration{
				Logging: logsv1.LoggingConfiguration{
					Options: logsv1.FormatOptions{
						Text: logsv1.TextOptions{
							OutputRoutingOptions: logsv1.OutputRoutingOptions{
								InfoBufferSize: resource.QuantityValue{
									// This will be set as default by the unmarshaler.
									Quantity: resource.MustParse("0"),
								},
							},
						},
						JSON: logsv1.JSONOptions{
							OutputRoutingOptions: logsv1.OutputRoutingOptions{
								InfoBufferSize: resource.QuantityValue{
									// This will be set as default by the unmarshaler.
									Quantity: resource.MustParse("0"),
								},
							},
						},
					},
				},
			},
			Konnectivity: Konnectivity{AgentPort: 1337},
		},
		map[string]string{
			"kubeletConfiguration": `{"syncFrequency":"0s","fileCheckFrequency":"0s","httpCheckFrequency":"0s","authentication":{"x509":{},"webhook":{"cacheTTL":"0s"},"anonymous":{}},"authorization":{"webhook":{"cacheAuthorizedTTL":"0s","cacheUnauthorizedTTL":"0s"}},"streamingConnectionIdleTimeout":"0s","nodeStatusUpdateFrequency":"0s","nodeStatusReportFrequency":"0s","imageMinimumGCAge":"0s","imageMaximumGCAge":"0s","volumeStatsAggPeriod":"0s","cpuManagerReconcilePeriod":"0s","runtimeRequestTimeout":"0s","evictionPressureTransitionPeriod":"0s","memorySwap":{},"logging":{"flushFrequency":0,"verbosity":0,"options":{"text":{"infoBufferSize":"0"},"json":{"infoBufferSize":"0"}}},"shutdownGracePeriod":"0s","shutdownGracePeriodCriticalPods":"0s","containerRuntimeEndpoint":""}`,
			"konnectivity":         `{"agentPort":1337}`,
		},
	},
	{
		"nllb",
		&Profile{
			NodeLocalLoadBalancing: &v1beta1.NodeLocalLoadBalancing{
				Enabled: true,
				Type:    v1beta1.NllbTypeEnvoyProxy,
				EnvoyProxy: &v1beta1.EnvoyProxy{
					Image: &v1beta1.ImageSpec{
						Image:   "example.com/image",
						Version: "latest",
					},
					ImagePullPolicy:            corev1.PullAlways,
					APIServerBindPort:          4711,
					KonnectivityServerBindPort: ptr.To(int32(1337)),
				},
			},
			Konnectivity: Konnectivity{AgentPort: 1337},
		},
		map[string]string{
			"nodeLocalLoadBalancing": `{"enabled":true,"type":"EnvoyProxy","envoyProxy":{"image":{"image":"example.com/image","version":"latest"},"imagePullPolicy":"Always","apiServerBindPort":4711,"konnectivityServerBindPort":1337}}`,
			"konnectivity":           `{"agentPort":1337}`,
		},
	},
	{
		"konnectivity",
		&Profile{
			Konnectivity: Konnectivity{Enabled: true, AgentPort: 1337},
		},
		map[string]string{
			"konnectivity": `{"enabled":true,"agentPort":1337}`,
		},
	},
}

func makeHostPort(host string, port uint16) net.HostPort {
	hostPort, err := net.NewHostPort(host, port)
	if err != nil {
		panic(err)
	}
	return *hostPort
}
