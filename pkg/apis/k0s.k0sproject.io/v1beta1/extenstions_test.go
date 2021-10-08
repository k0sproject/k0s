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
