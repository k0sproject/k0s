// SPDX-FileCopyrightText: 2022 k0s authors
// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"sync"
	"testing"
	"testing/synctest"
	"time"

	"github.com/k0sproject/k0s/pkg/apis/helm/v1beta1"
	k0sv1beta1 "github.com/k0sproject/k0s/pkg/apis/k0s/v1beta1"
	"github.com/k0sproject/k0s/pkg/component/controller/leaderelector"
	"github.com/k0sproject/k0s/pkg/config"
	"sigs.k8s.io/yaml"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/ptr"

	"github.com/k0sproject/k0s/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	clientgotesting "k8s.io/client-go/testing"
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

func TestExtensionsController_reconcilesCharts(t *testing.T) {
	tests := []struct {
		name  string
		chart k0sv1beta1.Chart
		want  string
	}{
		{
			name: "forceUpgrade is nil should omit from manifest",
			chart: k0sv1beta1.Chart{
				Name:      "release",
				ChartName: "k0s/chart",
				Version:   "0.0.1",
				Values:    "values",
				TargetNS:  metav1.NamespaceDefault,
				Timeout: k0sv1beta1.BackwardCompatibleDuration(
					metav1.Duration{Duration: 5 * time.Minute},
				),
			},
			want: `apiVersion: helm.k0sproject.io/v1beta1
kind: Chart
metadata:
  finalizers:
  - helm.k0sproject.io/uninstall-helm-release
  labels:
    k0s.k0sproject.io/stack: helm
  name: k0s-addon-chart-release
  namespace: ` + metav1.NamespaceSystem + `
spec:
  chartName: k0s/chart
  namespace: default
  releaseName: release
  timeout: 5m0s
  values: |2

    values
  version: 0.0.1
status: {}
`,
		},
		{
			name: "forceUpgrade is false should be included in manifest",
			chart: k0sv1beta1.Chart{
				Name:         "release",
				ChartName:    "k0s/chart",
				Version:      "0.0.1",
				Values:       "values",
				TargetNS:     metav1.NamespaceDefault,
				ForceUpgrade: ptr.To(false),
				Timeout: k0sv1beta1.BackwardCompatibleDuration(
					metav1.Duration{Duration: 5 * time.Minute},
				),
			},
			want: `apiVersion: helm.k0sproject.io/v1beta1
kind: Chart
metadata:
  finalizers:
  - helm.k0sproject.io/uninstall-helm-release
  labels:
    k0s.k0sproject.io/stack: helm
  name: k0s-addon-chart-release
  namespace: ` + metav1.NamespaceSystem + `
spec:
  chartName: k0s/chart
  forceUpgrade: false
  namespace: default
  releaseName: release
  timeout: 5m0s
  values: |2

    values
  version: 0.0.1
status: {}
`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			k0sVars, err := config.NewCfgVars(nil, t.TempDir())
			require.NoError(t, err)

			synctest.Test(t, func(t *testing.T) {
				cf := testutil.NewFakeClientFactory()

				// Automatically mark all added CRDs as established.
				cf.DynamicClient.PrependReactor("create", "customresourcedefinitions", func(action clientgotesting.Action) (handled bool, ret runtime.Object, err error) {
					crd := action.(clientgotesting.CreateAction).GetObject().(*unstructured.Unstructured)
					if err := unstructured.SetNestedSlice(crd.Object, []any{
						map[string]any{
							"type":   string(apiextensionsv1.Established),
							"status": string(apiextensionsv1.ConditionTrue),
						},
					}, "status", "conditions"); !assert.NoError(t, err) {
						return true, nil, err
					}

					return false, nil, nil
				})

				underTest := NewExtensionsController(k0sVars, cf, leaderelector.Off())

				// Spawn the config reconciliation goroutine.
				ctx := t.Context()
				reconciled := make(chan struct{})
				var wg sync.WaitGroup
				t.Cleanup(wg.Wait)
				wg.Go(func() { underTest.reconcileConfig(ctx, reconciled) })

				// Reconcile the chart via the cluster config.
				require.NoError(t, underTest.Reconcile(ctx, &k0sv1beta1.ClusterConfig{
					Spec: &k0sv1beta1.ClusterSpec{
						Extensions: &k0sv1beta1.ClusterExtensions{
							Helm: &k0sv1beta1.HelmExtensions{
								Charts: k0sv1beta1.ChartsSettings{tt.chart},
							},
						},
					},
				}))

				// Wait for the reconciliation goroutine to do its job.
				<-reconciled
				synctest.Wait()

				// Check that the chart has been written to the API server.
				charts, err := cf.K0sClient.HelmV1beta1().Charts(metav1.NamespaceSystem).List(ctx, metav1.ListOptions{})
				require.NoError(t, err)
				require.Len(t, charts.Items, 1)

				chart := &charts.Items[0]
				chart.Annotations = nil // we don't assert on annotations
				b, err := yaml.Marshal(chart)
				require.NoError(t, err)
				assert.Equal(t, tt.want, string(b))
			})
		})
	}
}
