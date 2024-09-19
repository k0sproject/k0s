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
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestValidation(t *testing.T) {
	t.Run("charts_validation", func(t *testing.T) {
		t.Run("name_is_empty", func(t *testing.T) {
			chart := Chart{
				Name:      "",
				ChartName: "k0s/chart",
				TargetNS:  "default",
			}
			assert.Error(t, chart.Validate())
		})
		t.Run("targetNS_is_empty", func(t *testing.T) {
			chart := Chart{
				Name:      "release",
				ChartName: "k0s/chart",
				TargetNS:  "",
			}
			assert.Error(t, chart.Validate())
		})
		t.Run("chartName_is_empty", func(t *testing.T) {
			chart := Chart{
				Name:      "release",
				ChartName: "",
				TargetNS:  "default",
			}
			assert.Error(t, chart.Validate())
		})
		t.Run("minimum_valid_chart", func(t *testing.T) {
			chart := Chart{
				Name:      "release",
				ChartName: "k0s/chart",
				TargetNS:  "default",
			}
			assert.NoError(t, chart.Validate())
		})
	})

	t.Run("repository_validation", func(t *testing.T) {
		t.Run("name_is_empty", func(t *testing.T) {
			repo := Repository{
				Name: "",
				URL:  "http://charts.helm.sh",
			}
			assert.Error(t, repo.Validate())
		})
		t.Run("url_is_empty", func(t *testing.T) {
			repo := Repository{
				Name: "repo",
			}
			assert.Error(t, repo.Validate())
		})
		t.Run("minimum_valid_repo", func(t *testing.T) {
			repo := Repository{
				Name: "repo",
				URL:  "http://charts.helm.sh",
			}
			assert.NoError(t, repo.Validate())
		})

	})
}

func TestIntegerTimeoutParsing(t *testing.T) {
	yaml := `
apiVersion: k0s.k0sproject.io/v1beta1
kind: ClusterConfig
metadata:
  name: foobar
spec:
  extensions:
    helm:
      charts:
      - name: prometheus-stack
        chartname: prometheus-community/prometheus
        version: "14.6.1"
        timeout: 60000000000
`

	c, err := ConfigFromString(yaml)
	require := require.New(t)

	require.NoError(err)

	chart := c.Spec.Extensions.Helm.Charts[0]
	expectedDuration := BackwardCompatibleDuration(
		metav1.Duration{Duration: time.Minute},
	)
	require.Equal(expectedDuration, chart.Timeout)
}

func TestDurationParsing(t *testing.T) {
	yaml := `
apiVersion: k0s.k0sproject.io/v1beta1
kind: ClusterConfig
metadata:
  name: foobar
spec:
  extensions:
    helm:
      charts:
      - name: prometheus-stack
        chartname: prometheus-community/prometheus
        version: "14.6.1"
        timeout: 20m
`

	c, err := ConfigFromString(yaml)
	require := require.New(t)

	require.NoError(err)

	chart := c.Spec.Extensions.Helm.Charts[0]
	expectedDuration := BackwardCompatibleDuration(
		metav1.Duration{Duration: 20 * time.Minute},
	)
	require.Equal(expectedDuration, chart.Timeout)
}
