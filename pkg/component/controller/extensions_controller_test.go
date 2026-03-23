// SPDX-FileCopyrightText: 2022 k0s authors
// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"path/filepath"
	"sync"
	"testing"
	"testing/synctest"
	"time"

	"github.com/k0sproject/k0s/pkg/apis/helm/v1beta1"
	k0sv1beta1 "github.com/k0sproject/k0s/pkg/apis/k0s/v1beta1"
	"github.com/k0sproject/k0s/pkg/component/controller/leaderelector"
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
  repository:
    url: https://charts.k0sproject.io/release
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
  repository:
    url: https://charts.k0sproject.io/release
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

				underTest := NewExtensionsController(cf, leaderelector.Off())

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
								// With cache, make these behave as in the old style
								// where config has the repo details,
								// ensuring backwards compatibility with the old style
								Repositories: k0sv1beta1.RepositoriesSettings{{
									Name: "k0s",
									URL:  "https://charts.k0sproject.io/release",
								}},
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

func TestExtractRepositoryIdentifier(t *testing.T) {
	tests := []struct {
		name      string
		chartName string
		want      string
	}{
		{
			name:      "OCI chart with registry host",
			chartName: "oci://ghcr.io/org/chart",
			want:      "ghcr.io",
		},
		{
			name:      "OCI chart with port",
			chartName: "oci://registry.io:5000/chart",
			want:      "registry.io:5000",
		},
		{
			name:      "OCI chart with path",
			chartName: "oci://registry.internal.com:8080/charts/app",
			want:      "registry.internal.com:8080",
		},
		{
			name:      "traditional chart",
			chartName: "myrepo/mychart",
			want:      "myrepo",
		},
		{
			name:      "traditional chart with version",
			chartName: "stable/nginx-ingress",
			want:      "stable",
		},
		{
			name:      "local relative path with dot",
			chartName: "./chart.tgz",
			want:      "",
		},
		{
			name:      "local relative path with dot-dot",
			chartName: "../chart.tgz",
			want:      "",
		},
		{
			name:      "chart name without slash",
			chartName: "nochart",
			want:      "",
		},
		{
			name:      "empty chart name",
			chartName: "",
			want:      "",
		},
		{
			name:      "malformed OCI URL",
			chartName: "oci://",
			want:      "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractRepositoryIdentifier(tt.chartName)
			assert.Equal(t, tt.want, got)
		})
	}

	// Test absolute path separately with platform-specific path
	t.Run("local absolute path", func(t *testing.T) {
		absPath, err := filepath.Abs("chart.tgz")
		require.NoError(t, err)
		got := extractRepositoryIdentifier(absPath)
		assert.Empty(t, got)
	})
}

func TestExtractOCIRegistryURL(t *testing.T) {
	tests := []struct {
		name      string
		chartName string
		want      string
	}{
		{
			name:      "OCI URL basic",
			chartName: "oci://ghcr.io/user/charts/app",
			want:      "oci://ghcr.io",
		},
		{
			name:      "OCI URL with port",
			chartName: "oci://registry:8080/chart",
			want:      "oci://registry:8080",
		},
		{
			name:      "OCI URL with subdomain",
			chartName: "oci://registry.internal.example.com/org/charts/mychart",
			want:      "oci://registry.internal.example.com",
		},
		{
			name:      "OCI URL with port and path",
			chartName: "oci://localhost:5000/charts/test",
			want:      "oci://localhost:5000",
		},
		{
			name:      "not OCI chart",
			chartName: "myrepo/chart",
			want:      "",
		},
		{
			name:      "empty string",
			chartName: "",
			want:      "",
		},
		{
			name:      "malformed OCI URL - no host",
			chartName: "oci://",
			want:      "",
		},
		{
			name:      "malformed OCI URL - invalid format",
			chartName: "oci:///no-host",
			want:      "",
		},
		{
			name:      "http URL not OCI",
			chartName: "https://charts.example.com/mychart",
			want:      "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractOCIRegistryURL(tt.chartName)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestExtractAndLookupRepository(t *testing.T) {
	// Setup repository cache
	repoCache := &repositoryCache{cache: make(map[string]k0sv1beta1.Repository)}
	repoCache.update([]k0sv1beta1.Repository{
		{
			Name:     "stable",
			URL:      "https://charts.helm.sh/stable",
			Username: "user1",
		},
		{
			Name:     "bitnami",
			URL:      "https://charts.bitnami.com/bitnami",
			Password: "pass1",
		},
		{
			Name: "ghcr.io", // OCI registry keyed by hostname
			URL:  "oci://ghcr.io",
		},
		{
			Name:     "registry.io:5000", // OCI registry with port
			URL:      "oci://registry.io:5000",
			Username: "robot",
			Password: "token",
		},
	})

	reconciler := &ChartReconciler{
		repositoryCache: repoCache,
	}

	tests := []struct {
		name      string
		chartName string
		wantRepo  *k0sv1beta1.Repository
		wantErr   bool
		errMsg    string
	}{
		{
			name:      "traditional chart with known repo",
			chartName: "stable/nginx",
			wantRepo: &k0sv1beta1.Repository{
				Name:     "stable",
				URL:      "https://charts.helm.sh/stable",
				Username: "user1",
			},
			wantErr: false,
		},
		{
			name:      "traditional chart with unknown repo",
			chartName: "unknown/chart",
			wantRepo:  nil,
			wantErr:   true,
			errMsg:    "repository 'unknown' not found",
		},
		{
			name:      "OCI chart with known registry",
			chartName: "oci://ghcr.io/org/chart",
			wantRepo: &k0sv1beta1.Repository{
				Name: "ghcr.io",
				URL:  "oci://ghcr.io",
			},
			wantErr: false,
		},
		{
			name:      "OCI chart with unknown registry - returns nil (anonymous access)",
			chartName: "oci://unknown.io/chart",
			wantRepo:  nil,
			wantErr:   false,
		},
		{
			name:      "OCI chart with port",
			chartName: "oci://registry.io:5000/charts/app",
			wantRepo: &k0sv1beta1.Repository{
				Name:     "registry.io:5000",
				URL:      "oci://registry.io:5000",
				Username: "robot",
				Password: "token",
			},
			wantErr: false,
		},
		{
			name:      "local relative path with dot",
			chartName: "./chart.tgz",
			wantRepo:  nil,
			wantErr:   false,
		},
		{
			name:      "local relative path with dot-dot",
			chartName: "../chart.tgz",
			wantRepo:  nil,
			wantErr:   false,
		},
		{
			name:      "invalid chart name without slash",
			chartName: "invalidchart",
			wantRepo:  nil,
			wantErr:   true,
			errMsg:    "expected format 'repository/chart'",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := reconciler.extractAndLookupRepository(tt.chartName)
			if tt.wantErr {
				require.Error(t, err)
				if tt.errMsg != "" {
					assert.Contains(t, err.Error(), tt.errMsg)
				}
			} else {
				require.NoError(t, err)
			}
			if tt.wantRepo == nil {
				assert.Nil(t, got)
			} else {
				require.NotNil(t, got)
				assert.Equal(t, tt.wantRepo.Name, got.Name)
				assert.Equal(t, tt.wantRepo.URL, got.URL)
				assert.Equal(t, tt.wantRepo.Username, got.Username)
				assert.Equal(t, tt.wantRepo.Password, got.Password)
			}
		})
	}

	// Test absolute path separately with platform-specific path
	t.Run("local absolute path", func(t *testing.T) {
		absPath, err := filepath.Abs("chart.tgz")
		require.NoError(t, err)
		got, err := reconciler.extractAndLookupRepository(absPath)
		require.NoError(t, err)
		assert.Nil(t, got)
	})
}
