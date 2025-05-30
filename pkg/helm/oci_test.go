/*
Copyright 2025 k0s authors

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

package helm

import (
	"testing"

	"github.com/stretchr/testify/require"
	"k8s.io/utils/ptr"

	"github.com/k0sproject/k0s/pkg/apis/k0s/v1beta1"
)

const (
	testRegistryHost           = "example.com"
	testRegistryHostPort       = "example.com:8080"
	testOCIRegistryURL         = "oci://" + testRegistryHost
	testOCIRegistryURLWithPort = "oci://" + testRegistryHostPort
	testOCIRegistryURLWithRepo = testOCIRegistryURL + "/my-repo"

	testCAPath   = "./testdata/tls/ca.crt"
	testCertPath = "./testdata/tls/tls.crt"
	testKeyPath  = "./testdata/tls/tls.key"
)

func TestOciRegistryManager_AddRegistry(t *testing.T) {
	testCases := []struct {
		name         string
		registryHost string
		repoCfg      v1beta1.Repository
	}{
		{
			name:         "Valid OCI Registry, no auth",
			registryHost: testRegistryHost,
			repoCfg: v1beta1.Repository{
				URL: testOCIRegistryURL,
			},
		},
		{
			name:         "Valid OCI Registry, basic auth",
			registryHost: testRegistryHost,
			repoCfg: v1beta1.Repository{
				URL:      testOCIRegistryURL,
				Username: "user",
				Password: "pass",
			},
		},
		{
			name:         "Valid OCI Registry, insecure",
			registryHost: testRegistryHost,
			repoCfg: v1beta1.Repository{
				URL:      testOCIRegistryURL,
				Insecure: ptr.To(true),
			},
		},
		{
			name:         "Valid OCI Registry with port",
			registryHost: testRegistryHostPort,
			repoCfg: v1beta1.Repository{
				URL: testOCIRegistryURLWithPort,
			},
		},
		{
			name:         "Valid OCI Registry with repo",
			registryHost: testRegistryHost,
			repoCfg: v1beta1.Repository{
				URL: testOCIRegistryURLWithRepo,
			},
		},
		{
			name:         "Valid OCI Registry; TLS auth",
			registryHost: testRegistryHost,
			repoCfg: v1beta1.Repository{
				URL:      testOCIRegistryURL,
				CertFile: testCertPath,
				KeyFile:  testKeyPath,
				CAFile:   testCAPath,
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			m := newOCIRegistryManager()

			err := m.AddRegistry(tc.repoCfg)
			require.NoError(t, err)

			_, exists := m.knownClients[tc.registryHost]
			require.True(t, exists, "registry has not been added to knownClients")
		})
	}
}

func TestOciRegistryManager_AddRegistryErrors(t *testing.T) {
	testCases := []struct {
		name        string
		repoCfg     v1beta1.Repository
		expectedErr string
	}{
		{
			name: "Invalid URL scheme",
			repoCfg: v1beta1.Repository{
				URL: "http://example.com",
			},
			expectedErr: "registry URL http://example.com is not an OCI registry",
		},
		{
			name: "Invalid CA file",
			repoCfg: v1beta1.Repository{
				URL:    testOCIRegistryURL,
				CAFile: "invalid/path/to/ca.crt",
			},
			expectedErr: "can't load CA file invalid/path/to/ca.crt",
		},
		{
			name: "Invalid certificate and key files",
			repoCfg: v1beta1.Repository{
				URL:      testOCIRegistryURL,
				CertFile: "invalid/path/to/tls.crt",
				KeyFile:  "invalid/path/to/tls.key",
			},
			expectedErr: "can't load certificate and key files",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			m := newOCIRegistryManager()

			err := m.AddRegistry(tc.repoCfg)
			require.Error(t, err)
			require.Contains(t, err.Error(), tc.expectedErr)
		})
	}
}

func TestOciRegistryManager_GetRegistryClient(t *testing.T) {
	testCases := []struct {
		name         string
		registryHost string
		// expectNil is false when we expect a new client to be created
		// because no client exists for the given registry host
		expectNil bool
	}{
		{
			name:         "Known registry without port",
			registryHost: testOCIRegistryURL,
			expectNil:    true,
		},
		{
			name:         "Known registry with port",
			registryHost: testOCIRegistryURLWithPort,
			expectNil:    true,
		},
		{
			name:         "Known registry with repo",
			registryHost: testOCIRegistryURLWithRepo,
			expectNil:    true,
		},
		{
			name:         "Unknown OCI registry",
			registryHost: "oci://unknown.com",
			expectNil:    false,
		},
		{
			name:         "Non-OCI registry",
			registryHost: "example.com",
			expectNil:    false,
		},
		{
			name:         "Classic repo/chart input",
			registryHost: "classic-repo/chart",
			expectNil:    false,
		},
	}

	m := newOCIRegistryManager()
	// set known clients to nils to differentiate between
	// known and new default clients returned for unknown registries
	m.knownClients[testRegistryHost] = nil
	m.knownClients[testRegistryHostPort] = nil

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
