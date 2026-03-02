// SPDX-FileCopyrightText: 2026 k0s authors
// SPDX-License-Identifier: Apache-2.0

package helm

import (
	"testing"

	"k8s.io/client-go/rest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRESTClientGetter_ImplementsClientConfig(t *testing.T) {
	cfg := rest.Config{Host: "https://does-not-matter.example.com"}
	underTest := restClientGetter{config: &cfg, namespace: "ns"}

	if disco, err := underTest.ToDiscoveryClient(); assert.NoError(t, err) {
		assert.NotNil(t, disco)
	}

	loader := underTest.ToRawKubeConfigLoader()
	require.NotNil(t, loader)

	if cfg, err := underTest.ClientConfig(); assert.NoError(t, err) {
		assert.Equal(t, "https://does-not-matter.example.com", cfg.Host)
	}

	if namespace, overridden, err := loader.Namespace(); assert.NoError(t, err) {
		assert.True(t, overridden)
		assert.Equal(t, "ns", namespace)
	}

	assert.NotPanics(t, func() { loader.ConfigAccess() })

	_, err := loader.RawConfig()
	assert.ErrorContains(t, err, "unsupported")
}

func TestRESTClientGetter_CachesDiscoveryClient(t *testing.T) {
	cfg := rest.Config{Host: "https://does-not-matter.example.com"}
	underTest := restClientGetter{config: &cfg, namespace: "ns"}

	c1, err := underTest.ToDiscoveryClient()
	require.NoError(t, err)
	c2, err := underTest.ToDiscoveryClient()
	require.NoError(t, err)
	assert.Same(t, c1, c2)
}

func TestRESTClientGetter_CachesToRESTMapper(t *testing.T) {
	cfg := rest.Config{Host: "https://does-not-matter.example.com"}
	underTest := restClientGetter{config: &cfg, namespace: "ns"}

	m1, err := underTest.ToRESTMapper()
	require.NoError(t, err)
	m2, err := underTest.ToRESTMapper()
	require.NoError(t, err)
	assert.Same(t, m1, m2)
}
