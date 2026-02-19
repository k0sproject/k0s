// SPDX-FileCopyrightText: 2022 k0s authors
// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	helmv1beta1 "github.com/k0sproject/k0s/pkg/apis/helm/v1beta1"
	k0sv1beta1 "github.com/k0sproject/k0s/pkg/apis/k0s/v1beta1"
	k0sscheme "github.com/k0sproject/k0s/pkg/client/clientset/scheme"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"

	helmchart "helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/release"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/sirupsen/logrus"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestChartNeedsUpgrade(t *testing.T) {
	var testCases = []struct {
		description string
		chart       helmv1beta1.Chart
		expected    bool
	}{
		{
			"no_changes",
			helmv1beta1.Chart{
				Spec: helmv1beta1.ChartSpec{
					ChartName:   "test",
					ReleaseName: "test-release",
					Values:      "",
					Version:     "0.0.1",
					Namespace:   "ns",
				},
				Status: helmv1beta1.ChartStatus{
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
			helmv1beta1.Chart{
				Spec: helmv1beta1.ChartSpec{
					ChartName:   "test",
					ReleaseName: "test-release",
					Values:      "new values",
					Version:     "0.0.1",
					Namespace:   "ns",
				},
				Status: helmv1beta1.ChartStatus{
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
			helmv1beta1.Chart{
				Spec: helmv1beta1.ChartSpec{
					ChartName:   "test",
					ReleaseName: "test-release",
					Values:      "",
					Version:     "0.0.2",
					Namespace:   "ns",
				},
				Status: helmv1beta1.ChartStatus{
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
			helmv1beta1.Chart{
				Spec: helmv1beta1.ChartSpec{
					ChartName:   "test",
					ReleaseName: "new-test-release",
					Values:      "",
					Version:     "0.0.1",
					Namespace:   "ns",
				},
				Status: helmv1beta1.ChartStatus{
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
			helmv1beta1.Chart{
				Spec: helmv1beta1.ChartSpec{
					ChartName:   "test",
					ReleaseName: "test-release",
					Values:      "",
					Version:     "0.0.1",
					Namespace:   "new-ns",
				},
				Status: helmv1beta1.ChartStatus{
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
		TargetNS:  metav1.NamespaceDefault,
	}

	chart1 := k0sv1beta1.Chart{
		Name:      "release",
		ChartName: "k0s/chart",
		TargetNS:  metav1.NamespaceDefault,
		Order:     1,
	}

	chart2 := k0sv1beta1.Chart{
		Name:      "release",
		ChartName: "k0s/chart",
		TargetNS:  metav1.NamespaceDefault,
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
					TargetNS:  metav1.NamespaceDefault,
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
  namespace: ` + metav1.NamespaceSystem + `
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
  repository:
    url: https://charts.k0sproject.io/release
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
					TargetNS:     metav1.NamespaceDefault,
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
  namespace: ` + metav1.NamespaceSystem + `
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
  repository:
    url: https://charts.k0sproject.io/release
`,
		},
	}
	for _, tt := range tests {
		// With cache, make these behave as in the old style where config has the repo details,
		// ensuring backwards compatibility with the old style
		repoCache := repositoryCache{cache: make(map[string]k0sv1beta1.Repository)}
		repoCache.update([]k0sv1beta1.Repository{
			{
				Name: "k0s",
				URL:  "https://charts.k0sproject.io/release",
			},
		})
		t.Run(tt.name, func(t *testing.T) {
			ec := &ExtensionsController{
				manifestsDir:    t.TempDir(),
				repositoryCache: &repoCache,
			}
			path, err := ec.writeChartManifestFile(tt.args.chart, tt.args.fileName)
			require.NoError(t, err)
			contents, err := os.ReadFile(path)
			require.NoError(t, err)
			assert.Equal(t, strings.TrimSpace(tt.want), strings.TrimSpace(string(contents)))
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

func TestExtensionsController_UpdateStatus(t *testing.T) {
	releaseResult := &release.Release{
		Name:      "new-release",
		Namespace: "target-ns",
		Version:   7,
		Chart: &helmchart.Chart{
			Metadata: &helmchart.Metadata{
				Version:    "1.2.3",
				AppVersion: "2.3.4",
			},
		},
	}

	for _, test := range []struct {
		name               string
		updateErr          error
		chartRelease       *release.Release
		storeObject        bool
		wantPatchError     bool
		wantReleaseName    string
		wantVersion        string
		wantAppVersion     string
		wantRevision       int64
		wantReleaseNS      string
		wantStatusErrorMsg string
	}{
		{
			name:               "successful reconciliation clears status error and preserves existing release fields",
			storeObject:        true,
			wantReleaseName:    "old-release",
			wantVersion:        "0.9.0",
			wantAppVersion:     "0.9.1",
			wantRevision:       3,
			wantReleaseNS:      "old-ns",
			wantStatusErrorMsg: "",
		},
		{
			name:               "failed reconciliation stores status error and preserves existing release fields",
			updateErr:          errors.New("boom"),
			storeObject:        true,
			wantReleaseName:    "old-release",
			wantVersion:        "0.9.0",
			wantAppVersion:     "0.9.1",
			wantRevision:       3,
			wantReleaseNS:      "old-ns",
			wantStatusErrorMsg: "boom",
		},
		{
			name:               "successful reconciliation maps release fields",
			chartRelease:       releaseResult,
			storeObject:        true,
			wantReleaseName:    releaseResult.Name,
			wantVersion:        releaseResult.Chart.Metadata.Version,
			wantAppVersion:     releaseResult.Chart.AppVersion(),
			wantRevision:       int64(releaseResult.Version),
			wantReleaseNS:      releaseResult.Namespace,
			wantStatusErrorMsg: "",
		},
		{
			name:           "status patch error is returned",
			storeObject:    false,
			wantPatchError: true,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			stored := &helmv1beta1.Chart{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-chart",
					Namespace: metav1.NamespaceSystem,
				},
				Spec: helmv1beta1.ChartSpec{
					ReleaseName: "release",
					Values:      "foo: bar",
				},
				Status: helmv1beta1.ChartStatus{
					ReleaseName: "old-release",
					Version:     "0.9.0",
					AppVersion:  "0.9.1",
					Revision:    3,
					Namespace:   "old-ns",
					Error:       "existing error",
					ValuesHash:  "existing-hash",
				},
			}

			// Pass a modified spec to verify status hash is derived from the reconciliation input.
			reconciled := stored.DeepCopy()
			reconciled.Spec.Values = "foo: updated"

			builder := fake.NewClientBuilder().
				WithScheme(k0sscheme.Scheme).
				WithStatusSubresource(&helmv1beta1.Chart{})
			if test.storeObject {
				builder = builder.WithObjects(stored)
			}
			c := builder.Build()

			cr := &ChartReconciler{
				Client: c,
				L:      logrus.NewEntry(logrus.New()),
			}

			err := cr.updateStatus(t.Context(), reconciled, test.chartRelease, test.updateErr)
			if test.wantPatchError {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)

			var got helmv1beta1.Chart
			require.NoError(t, c.Get(t.Context(), client.ObjectKeyFromObject(stored), &got))
			assert.Equal(t, reconciled.Spec.HashValues(), got.Status.ValuesHash)
			assert.Equal(t, test.wantReleaseName, got.Status.ReleaseName)
			assert.Equal(t, test.wantVersion, got.Status.Version)
			assert.Equal(t, test.wantAppVersion, got.Status.AppVersion)
			assert.Equal(t, test.wantRevision, got.Status.Revision)
			assert.Equal(t, test.wantReleaseNS, got.Status.Namespace)
			assert.Equal(t, test.wantStatusErrorMsg, got.Status.Error)
			assert.NotEmpty(t, got.Status.Updated)
		})
	}
}
