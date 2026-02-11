// SPDX-FileCopyrightText: 2026 k0s authors
// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"context"
	"testing"

	helmv1beta1 "github.com/k0sproject/k0s/pkg/apis/helm/v1beta1"
	"github.com/k0sproject/k0s/pkg/helm"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestLoadAndMergeRepositoryConfig(t *testing.T) {
	tests := []struct {
		name     string
		chart    helmv1beta1.Chart
		secret   *corev1.Secret // nil if secret doesn't exist
		wantRepo *helm.Repository
		wantErr  bool
		errMsg   string
	}{
		{
			name: "inline config only - traditional repo",
			chart: helmv1beta1.Chart{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-chart",
					Namespace: "kube-system",
				},
				Spec: helmv1beta1.ChartSpec{
					ChartName: "myrepo/chart",
					Repository: &helmv1beta1.RepositorySpec{
						URL:      "https://charts.example.com",
						Username: "inline-user",
					},
				},
			},
			wantRepo: &helm.Repository{
				Name:     "myrepo",
				URL:      "https://charts.example.com",
				Username: "inline-user",
			},
			wantErr: false,
		},
		{
			name: "secret overrides inline config",
			chart: helmv1beta1.Chart{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-chart",
					Namespace: "kube-system",
				},
				Spec: helmv1beta1.ChartSpec{
					ChartName: "myrepo/chart",
					Repository: &helmv1beta1.RepositorySpec{
						URL:      "https://inline.com",
						Username: "inline-user",
						Password: "inline-pass",
						ConfigFrom: &helmv1beta1.ConfigSource{
							SecretRef: &helmv1beta1.SecretReference{
								Name: "repo-secret",
							},
						},
					},
				},
			},
			secret: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "repo-secret",
					Namespace: "kube-system",
				},
				Data: map[string][]byte{
					"url":      []byte("https://secret.com"),
					"username": []byte("secret-user"),
					"password": []byte("secret-pass"),
				},
			},
			wantRepo: &helm.Repository{
				Name:     "myrepo",
				URL:      "https://secret.com",
				Username: "secret-user",
				Password: "secret-pass",
			},
			wantErr: false,
		},
		{
			name: "OCI chart with credentials in secret",
			chart: helmv1beta1.Chart{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "oci-chart",
					Namespace: "kube-system",
				},
				Spec: helmv1beta1.ChartSpec{
					ChartName: "oci://ghcr.io/org/chart",
					Repository: &helmv1beta1.RepositorySpec{
						ConfigFrom: &helmv1beta1.ConfigSource{
							SecretRef: &helmv1beta1.SecretReference{
								Name: "oci-secret",
							},
						},
					},
				},
			},
			secret: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "oci-secret",
					Namespace: "kube-system",
				},
				Data: map[string][]byte{
					"username": []byte("robot"),
					"password": []byte("token123"),
				},
			},
			wantRepo: &helm.Repository{
				Name:     "ghcr.io",
				Username: "robot",
				Password: "token123",
			},
			wantErr: false,
		},
		{
			name: "TLS certificates from secret",
			chart: helmv1beta1.Chart{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "tls-chart",
					Namespace: "kube-system",
				},
				Spec: helmv1beta1.ChartSpec{
					ChartName: "myrepo/chart",
					Repository: &helmv1beta1.RepositorySpec{
						URL: "https://secure.example.com",
						ConfigFrom: &helmv1beta1.ConfigSource{
							SecretRef: &helmv1beta1.SecretReference{
								Name: "tls-secret",
							},
						},
					},
				},
			},
			secret: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "tls-secret",
					Namespace: "kube-system",
				},
				Data: map[string][]byte{
					"ca.crt":  []byte("-----BEGIN CERTIFICATE-----\nCA_CERT_DATA\n-----END CERTIFICATE-----"),
					"tls.crt": []byte("-----BEGIN CERTIFICATE-----\nCLIENT_CERT_DATA\n-----END CERTIFICATE-----"),
					"tls.key": []byte("-----BEGIN PRIVATE KEY-----\nCLIENT_KEY_DATA\n-----END PRIVATE KEY-----"),
				},
			},
			wantRepo: &helm.Repository{
				Name:     "myrepo",
				URL:      "https://secure.example.com",
				CAData:   []byte("-----BEGIN CERTIFICATE-----\nCA_CERT_DATA\n-----END CERTIFICATE-----"),
				CertData: []byte("-----BEGIN CERTIFICATE-----\nCLIENT_CERT_DATA\n-----END CERTIFICATE-----"),
				KeyData:  []byte("-----BEGIN PRIVATE KEY-----\nCLIENT_KEY_DATA\n-----END PRIVATE KEY-----"),
			},
			wantErr: false,
		},
		{
			name: "secret with insecure flag",
			chart: helmv1beta1.Chart{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "insecure-chart",
					Namespace: "kube-system",
				},
				Spec: helmv1beta1.ChartSpec{
					ChartName: "myrepo/chart",
					Repository: &helmv1beta1.RepositorySpec{
						URL: "https://insecure.example.com",
						ConfigFrom: &helmv1beta1.ConfigSource{
							SecretRef: &helmv1beta1.SecretReference{
								Name: "insecure-secret",
							},
						},
					},
				},
			},
			secret: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "insecure-secret",
					Namespace: "kube-system",
				},
				Data: map[string][]byte{
					"insecure": []byte("true"),
				},
			},
			wantRepo: &helm.Repository{
				Name:     "myrepo",
				URL:      "https://insecure.example.com",
				Insecure: ptr.To(true),
			},
			wantErr: false,
		},
		{
			name: "secret in different namespace",
			chart: helmv1beta1.Chart{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "cross-ns-chart",
					Namespace: "kube-system",
				},
				Spec: helmv1beta1.ChartSpec{
					ChartName: "myrepo/chart",
					Repository: &helmv1beta1.RepositorySpec{
						URL: "https://example.com",
						ConfigFrom: &helmv1beta1.ConfigSource{
							SecretRef: &helmv1beta1.SecretReference{
								Name:      "repo-secret",
								Namespace: "default",
							},
						},
					},
				},
			},
			secret: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "repo-secret",
					Namespace: "default",
				},
				Data: map[string][]byte{
					"username": []byte("cross-ns-user"),
				},
			},
			wantRepo: &helm.Repository{
				Name:     "myrepo",
				URL:      "https://example.com",
				Username: "cross-ns-user",
			},
			wantErr: false,
		},
		{
			name: "inline file paths with secret data - data takes precedence",
			chart: helmv1beta1.Chart{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "mixed-chart",
					Namespace: "kube-system",
				},
				Spec: helmv1beta1.ChartSpec{
					ChartName: "myrepo/chart",
					Repository: &helmv1beta1.RepositorySpec{
						URL:      "https://example.com",
						CAFile:   "/path/to/ca.crt",
						CertFile: "/path/to/cert.crt",
						KeyFile:  "/path/to/key.key",
						ConfigFrom: &helmv1beta1.ConfigSource{
							SecretRef: &helmv1beta1.SecretReference{
								Name: "data-secret",
							},
						},
					},
				},
			},
			secret: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "data-secret",
					Namespace: "kube-system",
				},
				Data: map[string][]byte{
					"ca.crt": []byte("CA_DATA"),
				},
			},
			wantRepo: &helm.Repository{
				Name:     "myrepo",
				URL:      "https://example.com",
				CAData:   []byte("CA_DATA"),
				CAFile:   "", // File path cleared when data is provided
				CertFile: "/path/to/cert.crt",
				KeyFile:  "/path/to/key.key",
			},
			wantErr: false,
		},
		{
			name: "partial secret data - inline values preserved",
			chart: helmv1beta1.Chart{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "partial-chart",
					Namespace: "kube-system",
				},
				Spec: helmv1beta1.ChartSpec{
					ChartName: "myrepo/chart",
					Repository: &helmv1beta1.RepositorySpec{
						URL:      "https://inline.com",
						Username: "inline-user",
						Password: "inline-pass",
						ConfigFrom: &helmv1beta1.ConfigSource{
							SecretRef: &helmv1beta1.SecretReference{
								Name: "partial-secret",
							},
						},
					},
				},
			},
			secret: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "partial-secret",
					Namespace: "kube-system",
				},
				Data: map[string][]byte{
					"password": []byte("secret-pass"), // Only override password
				},
			},
			wantRepo: &helm.Repository{
				Name:     "myrepo",
				URL:      "https://inline.com",
				Username: "inline-user", // from inline
				Password: "secret-pass", // from secret
			},
			wantErr: false,
		},
		{
			name: "secret not found",
			chart: helmv1beta1.Chart{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "missing-secret-chart",
					Namespace: "kube-system",
				},
				Spec: helmv1beta1.ChartSpec{
					ChartName: "myrepo/chart",
					Repository: &helmv1beta1.RepositorySpec{
						ConfigFrom: &helmv1beta1.ConfigSource{
							SecretRef: &helmv1beta1.SecretReference{
								Name: "missing-secret",
							},
						},
					},
				},
			},
			secret:  nil,
			wantErr: true,
			errMsg:  "failed to get repository config secret",
		},
		{
			name: "empty secret values are ignored",
			chart: helmv1beta1.Chart{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "empty-values-chart",
					Namespace: "kube-system",
				},
				Spec: helmv1beta1.ChartSpec{
					ChartName: "myrepo/chart",
					Repository: &helmv1beta1.RepositorySpec{
						URL:      "https://inline.com",
						Username: "inline-user",
						ConfigFrom: &helmv1beta1.ConfigSource{
							SecretRef: &helmv1beta1.SecretReference{
								Name: "empty-secret",
							},
						},
					},
				},
			},
			secret: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "empty-secret",
					Namespace: "kube-system",
				},
				Data: map[string][]byte{
					"username": []byte(""), // Empty value should not override
					"password": []byte("secret-pass"),
				},
			},
			wantRepo: &helm.Repository{
				Name:     "myrepo",
				URL:      "https://inline.com",
				Username: "inline-user", // preserved from inline
				Password: "secret-pass", // from secret
			},
			wantErr: false,
		},
		{
			name: "no repository spec",
			chart: helmv1beta1.Chart{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "no-repo-chart",
					Namespace: "kube-system",
				},
				Spec: helmv1beta1.ChartSpec{
					ChartName:  "myrepo/chart",
					Repository: nil,
				},
			},
			wantErr: true,
			errMsg:  "chart has no repository configuration",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Build scheme with Chart CRD
			scheme := runtime.NewScheme()
			_ = helmv1beta1.AddToScheme(scheme)
			_ = corev1.AddToScheme(scheme)

			// Create fake client with optional secret
			var objects []runtime.Object
			if tt.secret != nil {
				objects = append(objects, tt.secret)
			}
			fakeClient := fake.NewClientBuilder().
				WithScheme(scheme).
				WithRuntimeObjects(objects...).
				Build()

			reconciler := &ChartReconciler{
				Client: fakeClient,
			}

			ctx := context.Background()
			got, err := reconciler.loadAndMergeRepositoryConfig(ctx, tt.chart)

			if tt.wantErr {
				require.Error(t, err)
				if tt.errMsg != "" {
					assert.Contains(t, err.Error(), tt.errMsg)
				}
				return
			}

			require.NoError(t, err)
			require.NotNil(t, got)

			// Verify repository fields
			assert.Equal(t, tt.wantRepo.Name, got.Name, "Name mismatch")
			assert.Equal(t, tt.wantRepo.URL, got.URL, "URL mismatch")
			assert.Equal(t, tt.wantRepo.Username, got.Username, "Username mismatch")
			assert.Equal(t, tt.wantRepo.Password, got.Password, "Password mismatch")
			assert.Equal(t, tt.wantRepo.CAFile, got.CAFile, "CAFile mismatch")
			assert.Equal(t, tt.wantRepo.CertFile, got.CertFile, "CertFile mismatch")
			assert.Equal(t, tt.wantRepo.KeyFile, got.KeyFile, "KeyFile mismatch")
			assert.Equal(t, tt.wantRepo.CAData, got.CAData, "CAData mismatch")
			assert.Equal(t, tt.wantRepo.CertData, got.CertData, "CertData mismatch")
			assert.Equal(t, tt.wantRepo.KeyData, got.KeyData, "KeyData mismatch")

			if tt.wantRepo.Insecure != nil {
				require.NotNil(t, got.Insecure, "Insecure should not be nil")
				assert.Equal(t, *tt.wantRepo.Insecure, *got.Insecure, "Insecure mismatch")
			} else {
				assert.Nil(t, got.Insecure, "Insecure should be nil")
			}
		})
	}
}
