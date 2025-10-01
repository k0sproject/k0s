// SPDX-FileCopyrightText: 2022 k0s authors
// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"os"
	"strings"
	"testing"
	"time"

	"github.com/k0sproject/k0s/pkg/apis/helm/v1beta1"
	k0sv1beta1 "github.com/k0sproject/k0s/pkg/apis/k0s/v1beta1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
)

func TestChartNeedsUpgrade(t *testing.T) {
	var testCases = []struct {
		description string
		chart       v1beta1.Chart
		expected    bool
	}{
		{
			"no_changes",
			v1beta1.Chart{
				Spec: v1beta1.ChartSpec{
					ChartName:   "test",
					ReleaseName: "test-release",
					Values:      "",
					Version:     "0.0.1",
					Namespace:   "ns",
				},
				Status: v1beta1.ChartStatus{
					ReleaseName: "test-release",
					Version:     "0.0.1",
					Namespace:   "ns",
					ValuesHash:  "41c7250e092d11639c77c823fb34afa232c5ceb316ad546b4df506606ef9b3e4",
				},
			},
			false,
		},
		{
			"changed_values",
			v1beta1.Chart{
				Spec: v1beta1.ChartSpec{
					ChartName:   "test",
					ReleaseName: "test-release",
					Values:      "new values",
					Version:     "0.0.1",
					Namespace:   "ns",
				},
				Status: v1beta1.ChartStatus{
					ReleaseName: "test-release",
					Version:     "0.0.1",
					Namespace:   "ns",
					ValuesHash:  "41c7250e092d11639c77c823fb34afa232c5ceb316ad546b4df506606ef9b3e4",
				},
			},
			true,
		},
		{
			"changed_chart_version",
			v1beta1.Chart{
				Spec: v1beta1.ChartSpec{
					ChartName:   "test",
					ReleaseName: "test-release",
					Values:      "",
					Version:     "0.0.2",
					Namespace:   "ns",
				},
				Status: v1beta1.ChartStatus{
					ReleaseName: "test-release",
					Version:     "0.0.1",
					Namespace:   "ns",
					ValuesHash:  "41c7250e092d11639c77c823fb34afa232c5ceb316ad546b4df506606ef9b3e4",
				},
			},
			true,
		},
		{
			"changed_release_name",
			v1beta1.Chart{
				Spec: v1beta1.ChartSpec{
					ChartName:   "test",
					ReleaseName: "new-test-release",
					Values:      "",
					Version:     "0.0.1",
					Namespace:   "ns",
				},
				Status: v1beta1.ChartStatus{
					ReleaseName: "test-release",
					Version:     "0.0.1",
					Namespace:   "ns",
					ValuesHash:  "41c7250e092d11639c77c823fb34afa232c5ceb316ad546b4df506606ef9b3e4",
				},
			},
			true,
		},
		{
			"changed_namespace",
			v1beta1.Chart{
				Spec: v1beta1.ChartSpec{
					ChartName:   "test",
					ReleaseName: "test-release",
					Values:      "",
					Version:     "0.0.1",
					Namespace:   "new-ns",
				},
				Status: v1beta1.ChartStatus{
					ReleaseName: "test-release",
					Version:     "0.0.1",
					Namespace:   "ns",
					ValuesHash:  "41c7250e092d11639c77c823fb34afa232c5ceb316ad546b4df506606ef9b3e4",
				},
			},
			true,
		},
	}

	cr := new(ChartReconciler)
	for _, tc := range testCases {
		t.Run(tc.description, func(t *testing.T) {
			actual := cr.chartNeedsUpgrade(tc.chart)
			assert.Equal(t, tc.expected, actual)
		})
	}
}

func TestChartManifestFileName(t *testing.T) {
	chart := k0sv1beta1.Chart{
		Name:      "release",
		ChartName: "k0s/chart",
		TargetNS:  "default",
	}

	chart1 := k0sv1beta1.Chart{
		Name:      "release",
		ChartName: "k0s/chart",
		TargetNS:  "default",
		Order:     1,
	}

	chart2 := k0sv1beta1.Chart{
		Name:      "release",
		ChartName: "k0s/chart",
		TargetNS:  "default",
		Order:     2,
	}

	assert.Equal(t, "0_helm_extension_release.yaml", chartManifestFileName(&chart))
	assert.Equal(t, "1_helm_extension_release.yaml", chartManifestFileName(&chart1))
	assert.Equal(t, "2_helm_extension_release.yaml", chartManifestFileName(&chart2))
	assert.True(t, isChartManifestFileName("0_helm_extension_release.yaml"))
}

func TestExtensionsController_writeChartManifestFile(t *testing.T) {
	type args struct {
		chart    k0sv1beta1.Chart
		fileName string
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "forceUpgrade is nil should omit from manifest",
			args: args{
				chart: k0sv1beta1.Chart{
					Name:      "release",
					ChartName: "k0s/chart",
					Version:   "0.0.1",
					Values:    "values",
					TargetNS:  "default",
					Timeout: k0sv1beta1.BackwardCompatibleDuration(
						metav1.Duration{Duration: 5 * time.Minute},
					),
				},
				fileName: "0_helm_extension_release.yaml",
			},
			want: `apiVersion: helm.k0sproject.io/v1beta1
kind: Chart
metadata:
  name: k0s-addon-chart-release
  namespace: "kube-system"
  finalizers:
    - helm.k0sproject.io/uninstall-helm-release
spec:
  chartName: k0s/chart
  releaseName: release
  timeout: 5m0s
  values: |

    values
  version: 0.0.1
  namespace: default
`,
		},
		{
			name: "forceUpgrade is false should be included in manifest",
			args: args{
				chart: k0sv1beta1.Chart{
					Name:         "release",
					ChartName:    "k0s/chart",
					Version:      "0.0.1",
					Values:       "values",
					TargetNS:     "default",
					ForceUpgrade: ptr.To(false),
					Timeout: k0sv1beta1.BackwardCompatibleDuration(
						metav1.Duration{Duration: 5 * time.Minute},
					),
				},
				fileName: "0_helm_extension_release.yaml",
			},
			want: `apiVersion: helm.k0sproject.io/v1beta1
kind: Chart
metadata:
  name: k0s-addon-chart-release
  namespace: "kube-system"
  finalizers:
    - helm.k0sproject.io/uninstall-helm-release
spec:
  chartName: k0s/chart
  releaseName: release
  timeout: 5m0s
  values: |

    values
  version: 0.0.1
  namespace: default
  forceUpgrade: false
`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ec := &ExtensionsController{
				manifestsDir: t.TempDir(),
			}
			path, err := ec.writeChartManifestFile(tt.args.chart, tt.args.fileName)
			require.NoError(t, err)
			contents, err := os.ReadFile(path)
			require.NoError(t, err)
			assert.Equal(t, strings.TrimSpace(tt.want), strings.TrimSpace(string(contents)))
		})
	}
}
