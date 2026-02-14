// SPDX-FileCopyrightText: 2025 k0s authors
// SPDX-License-Identifier: Apache-2.0

package helm

import (
	"os"
	"path"
	"testing"

	"github.com/cloudflare/cfssl/csr"
	"github.com/cloudflare/cfssl/initca"
	"github.com/stretchr/testify/require"
	"k8s.io/utils/ptr"
)

const (
	testRegistryHost           = "example.com"
	testRegistryHostPort       = "example.com:8080"
	testOCIRegistryURL         = "oci://" + testRegistryHost
	testOCIRegistryURLWithPort = "oci://" + testRegistryHostPort
	testOCIRegistryURLWithRepo = testOCIRegistryURL + "/my-repo"

	caCertFilename = "ca.crt"
	caKeyFilename  = "ca.key"
)

func initCA(t *testing.T, certsDir string) {
	var err error
	var keyData []byte
	certData, _, keyData, err := initca.New(&csr.CertificateRequest{
		KeyRequest: csr.NewKeyRequest(),
		CN:         "Test CA",
	})
	require.NoError(t, err)

	require.NoError(t, os.WriteFile(path.Join(certsDir, caCertFilename), certData, 0644))
	require.NoError(t, os.WriteFile(path.Join(certsDir, caKeyFilename), keyData, 0600))
}

func TestOCIRegistryManager_AddRegistry(t *testing.T) {
	testCases := []struct {
		name         string
		registryHost string
		repoCfg      Repository
		expectedErr  string
	}{
		{
			name:         "Valid OCI Registry URL",
			registryHost: testRegistryHost,
			repoCfg: Repository{
				URL: testOCIRegistryURL,
			},
		},
		{
			name:         "Valid OCI Registry URL with trailing slash",
			registryHost: testRegistryHost,
			repoCfg: Repository{
				URL: testOCIRegistryURL + "/",
			},
		},
		{
			name:         "Valid OCI Registry URL with port",
			registryHost: testRegistryHostPort,
			repoCfg: Repository{
				URL: testOCIRegistryURLWithPort,
			},
		},
		{
			name: "Invalid URL scheme",
			repoCfg: Repository{
				URL: "http://example.com",
			},
			expectedErr: "not an OCI registry",
		},
		{
			name: "Invalid URL format",
			repoCfg: Repository{
				URL: "oci://\\//\\//",
			},
			expectedErr: "can't parse repository URL",
		},
		{
			name: "Valid OCI Registry with path",
			repoCfg: Repository{
				URL: testOCIRegistryURLWithRepo,
			},
			expectedErr: "must not contain a path",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			m := newOCIRegistryManager()

			err := m.AddRegistry(tc.repoCfg)

			if tc.expectedErr != "" {
				require.Error(t, err)
				require.Contains(t, err.Error(), tc.expectedErr)
				return
			}

			require.NoError(t, err)
			c, exists := m.knownRegistries.Load(tc.registryHost)
			require.True(t, exists, "registry has not been added to knownRegistries")

			_, ok := c.(Repository)
			require.True(t, ok, "expected Repository type, got %T", c)
		})
	}
}

func TestOCIRegistryManager_GetRegistryClientErrors(t *testing.T) {
	testCases := []struct {
		name        string
		repoCfg     Repository
		expectedErr string
	}{
		{
			name: "Invalid CA file",
			repoCfg: Repository{
				URL:    testOCIRegistryURL,
				CAFile: "invalid/path/to/ca.crt",
			},
			expectedErr: "can't load CA file invalid/path/to/ca.crt",
		},
		{
			name: "mTLS client cert and key files",
			repoCfg: Repository{
				URL:      testOCIRegistryURL,
				CertFile: "path/to/client.crt",
				KeyFile:  "path/to/client.key",
			},
			expectedErr: "can't load client certificate",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			m := newOCIRegistryManager()
			require.NoError(t, m.AddRegistry(tc.repoCfg))

			_, err := m.GetRegistryClient(tc.repoCfg.URL)
			require.ErrorContains(t, err, tc.expectedErr)
		})
	}
}

func TestOCIRegistryManager_GetRegistryClient_URL(t *testing.T) {
	testCases := []struct {
		name         string
		registryHost string
		// expectNil is true when there is no known client for the registry,
		expectNil bool
	}{
		{
			name:         "Known registry without port",
			registryHost: testOCIRegistryURL,
			expectNil:    false,
		},
		{
			name:         "Known registry with port",
			registryHost: testOCIRegistryURLWithPort,
			expectNil:    false,
		},
		{
			name:         "Known registry with repo",
			registryHost: testOCIRegistryURLWithRepo,
			expectNil:    false,
		},
		{
			name:         "Unknown OCI registry",
			registryHost: "oci://unknown.com",
			expectNil:    true,
		},
		{
			name:         "Non-OCI registry",
			registryHost: "example.com",
			expectNil:    true,
		},
		{
			name:         "Classic repo/chart input",
			registryHost: "classic-repo/chart",
			expectNil:    true,
		},
	}

	m := newOCIRegistryManager()
	require.NoError(t, m.AddRegistry(Repository{URL: testOCIRegistryURL}))
	require.NoError(t, m.AddRegistry(Repository{URL: testOCIRegistryURLWithPort}))

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			client, err := m.GetRegistryClient(tc.registryHost)
			require.NoError(t, err)

			if tc.expectNil {
				require.Nil(t, client)
			} else {
				require.NotNil(t, client)
			}
		})
	}
}

func TestOCIRegistryManager_GetRegistryClient_Settings(t *testing.T) {
	certsDir := t.TempDir()
	initCA(t, certsDir)

	testCases := []struct {
		name    string
		repoCfg Repository
	}{
		{
			name: "Valid OCI Registry, basic auth",
			repoCfg: Repository{
				URL:      testOCIRegistryURL,
				Username: "user",
				Password: "pass",
			},
		},
		{
			name: "Valid OCI Registry, insecure",
			repoCfg: Repository{
				URL:      testOCIRegistryURL,
				Insecure: ptr.To(true),
			},
		},
		{
			name: "Valid OCI Registry with self-signed CA cert",
			repoCfg: Repository{
				URL:    testOCIRegistryURL,
				CAFile: path.Join(certsDir, caCertFilename),
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			m := newOCIRegistryManager()
			require.NoError(t, m.AddRegistry(tc.repoCfg))

			client, err := m.GetRegistryClient(tc.repoCfg.URL)
			require.NoError(t, err)
			require.NotNil(t, client)
		})
	}
}

func TestOCIRegistryManager_mTLS_CertWithoutKeyFails(t *testing.T) {
	m := newOCIRegistryManager()

	repoCfg := Repository{
		Name:     "test-repo",
		URL:      testOCIRegistryURL,
		CertFile: "/some/path/client.crt",
	}

	require.NoError(t, m.AddRegistry(repoCfg))

	_, err := m.GetRegistryClient(repoCfg.URL)
	require.ErrorContains(t, err, "must set both certFile and keyFile")
}

func TestOCIRegistryManager_mTLS_KeyWithoutCertFails(t *testing.T) {
	m := newOCIRegistryManager()

	repoCfg := Repository{
		Name:    "test-repo",
		URL:     testOCIRegistryURL,
		KeyFile: "/some/path/client.key",
	}

	require.NoError(t, m.AddRegistry(repoCfg))

	_, err := m.GetRegistryClient(repoCfg.URL)
	require.ErrorContains(t, err, "must set both certFile and keyFile")
}

func TestOCIRegistryManager_mTLS_InvalidCertKeyFails(t *testing.T) {
	m := newOCIRegistryManager()

	repoCfg := Repository{
		Name:     "test-repo",
		URL:      testOCIRegistryURL,
		CertFile: "/invalid/client.crt",
		KeyFile:  "/invalid/client.key",
	}

	require.NoError(t, m.AddRegistry(repoCfg))

	_, err := m.GetRegistryClient(repoCfg.URL)
	require.ErrorContains(t, err, "can't load client certificate")
}

func TestOCIRegistryManager_mTLS_Success(t *testing.T) {
	certsDir := t.TempDir()
	initCA(t, certsDir)

	// reuse CA cert as dummy client cert
	clientCert := path.Join(certsDir, caCertFilename)
	clientKey := path.Join(certsDir, caKeyFilename)

	m := newOCIRegistryManager()

	repoCfg := Repository{
		Name:     "test-repo",
		URL:      testOCIRegistryURL,
		CAFile:   clientCert,
		CertFile: clientCert,
		KeyFile:  clientKey,
	}

	require.NoError(t, m.AddRegistry(repoCfg))

	client, err := m.GetRegistryClient(repoCfg.URL)
	require.NoError(t, err)
	require.NotNil(t, client)
}
