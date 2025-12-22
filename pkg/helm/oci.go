// SPDX-FileCopyrightText: 2025 k0s authors
// SPDX-License-Identifier: Apache-2.0

package helm

import (
	"crypto/tls"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"sync"

	"helm.sh/helm/v3/pkg/registry"
	"k8s.io/client-go/util/cert"

	"github.com/k0sproject/k0s/pkg/apis/k0s/v1beta1"
)

// ociRegistryManager is a helper struct for managing OCI registry clients
type ociRegistryManager struct {
	knownRegistries sync.Map
}

func newOCIRegistryManager() *ociRegistryManager {
	return &ociRegistryManager{}
}

// AddRegistry adds a new registry client for the given repository configuration
func (m *ociRegistryManager) AddRegistry(repoCfg v1beta1.Repository) error {
	if !registry.IsOCI(repoCfg.URL) {
		return fmt.Errorf("%w: not an OCI registry", errors.ErrUnsupported)
	}

	registryURL, err := url.Parse(repoCfg.URL)
	if err != nil {
		return fmt.Errorf("can't parse repository URL: %w", err)
	}

	if registryURL.Path != "" && registryURL.Path != "/" {
		// to make future mTLS support easier, we require that the URL does not contain a path
		// see https://github.com/k0sproject/k0s/pull/5901#issuecomment-2943007708
		return fmt.Errorf("repository URL %s must not contain a path", repoCfg.URL)
	}

	m.knownRegistries.Store(registryURL.Host, repoCfg)
	return nil
}

// GetRegistryClient returns a registry client for a previously added registry URL or
// nil if the URL is not known.
func (m *ociRegistryManager) GetRegistryClient(rawRegistryURL string) (*registry.Client, error) {
	registryURL, err := url.Parse(rawRegistryURL)
	if err != nil {
		return nil, nil
	}

	repoCfgValue, exists := m.knownRegistries.Load(registryURL.Host)
	if !exists {
		return nil, nil
	}

	repoCfg, ok := repoCfgValue.(v1beta1.Repository)
	if !ok {
		return nil, fmt.Errorf("stored repository configuration for %s is not of type v1beta1.Repository", registryURL.Host)
	}

	tlsConfig := &tls.Config{
		InsecureSkipVerify: repoCfg.Insecure != nil && *repoCfg.Insecure,
	}
	if repoCfg.CAFile != "" {
		cPool, err := cert.NewPool(repoCfg.CAFile)
		if err != nil {
			return nil, fmt.Errorf("can't load CA file %s: %w", repoCfg.CAFile, err)
		}
		tlsConfig.RootCAs = cPool
	}
	if repoCfg.CertFile != "" || repoCfg.KeyFile != "" {
		if repoCfg.CertFile == "" || repoCfg.KeyFile == "" {
			return nil, fmt.Errorf("repository %s must set both certFile and keyFile for mTLS", repoCfg.Name)
		}
		clientCert, err := tls.LoadX509KeyPair(repoCfg.CertFile, repoCfg.KeyFile)
		if err != nil {
			return nil, fmt.Errorf("can't load client certificate for repository %s: %w", repoCfg.Name, err)
		}
		tlsConfig.Certificates = []tls.Certificate{clientCert}
	}

	// add applicable options from the Helm's default registry client
	// https://github.com/helm/helm/blob/7031000b7d4fb7d780441749c2f47777cc782bb3/pkg/cmd/root.go#L362
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
		return nil, fmt.Errorf("can't create registry client for repository %s: %w", repoCfg.Name, err)
	}

	return registryClient, nil
}
