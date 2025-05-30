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
	"crypto/tls"
	"fmt"
	"net/http"
	"net/url"
	"os"

	"helm.sh/helm/v3/pkg/registry"
	"k8s.io/client-go/util/cert"

	"github.com/k0sproject/k0s/pkg/apis/k0s/v1beta1"
)

// ociRegistryManager is a helper struct for managing OCI registry clients
type ociRegistryManager struct {
	knownClients map[string]*registry.Client
}

func newOCIRegistryManager() *ociRegistryManager {
	return &ociRegistryManager{
		knownClients: make(map[string]*registry.Client),
	}
}

// AddRegistry adds a new registry client for the given repository configuration
func (m *ociRegistryManager) AddRegistry(repoCfg v1beta1.Repository) error {
	registryURL, err := url.Parse(repoCfg.URL)
	if err != nil {
		return fmt.Errorf("can't parse registry URL %s: %w", repoCfg.URL, err)
	}
	if registryURL.Scheme != "oci" {
		return fmt.Errorf("registry URL %s is not an OCI registry", repoCfg.URL)
	}

	tlsConfig := &tls.Config{
		InsecureSkipVerify: repoCfg.Insecure != nil && *repoCfg.Insecure,
	}
	if repoCfg.CAFile != "" {
		cPool, err := cert.NewPool(repoCfg.CAFile)
		if err != nil {
			return fmt.Errorf("can't load CA file %s: %w", repoCfg.CAFile, err)
		}
		tlsConfig.RootCAs = cPool
	}
	if repoCfg.CertFile != "" && repoCfg.KeyFile != "" {
		keyPair, err := tls.LoadX509KeyPair(repoCfg.CertFile, repoCfg.KeyFile)
		if err != nil {
			return fmt.Errorf("can't load certificate and key files: %w", err)
		}
		tlsConfig.Certificates = []tls.Certificate{keyPair}
	}

	registryClient, err := registry.NewClient(
		registry.ClientOptWriter(os.Stdout),
		registry.ClientOptEnableCache(true),
		registry.ClientOptBasicAuth(repoCfg.Username, repoCfg.Password),
		registry.ClientOptHTTPClient(&http.Client{
			Transport: &http.Transport{
				TLSClientConfig: tlsConfig,
				Proxy:           http.ProxyFromEnvironment,
			},
		}),
	)
	if err != nil {
		return fmt.Errorf("can't create registry client for registry %s: %w", repoCfg.URL, err)
	}

	m.knownClients[registryURL.Host] = registryClient
	return nil
}

func (m *ociRegistryManager) getDefaultRegistryClient() (*registry.Client, error) {
	return registry.NewClient(
		registry.ClientOptWriter(os.Stdout),
		registry.ClientOptEnableCache(true),
	)
}

// GetRegistryClient returns a registry client for a previously added registry URL or
// a default registry client if the URL is not known.
func (m *ociRegistryManager) GetRegistryClient(rawRegistryURL string) (*registry.Client, error) {
	registryURL, err := url.Parse(rawRegistryURL)
	if err != nil {
		return m.getDefaultRegistryClient()
	}

	client, exists := m.knownClients[registryURL.Host]
	if exists {
		return client, nil
	}

	return m.getDefaultRegistryClient()
}
