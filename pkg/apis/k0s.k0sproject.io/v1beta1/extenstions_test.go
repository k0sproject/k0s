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
