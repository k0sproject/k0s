// SPDX-FileCopyrightText: 2026 k0s authors
// SPDX-License-Identifier: Apache-2.0

package clusterinfo

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/k0sproject/k0s/internal/testutil"
	"github.com/k0sproject/k0s/pkg/apis/k0s/v1beta1"
	"github.com/k0sproject/k0s/pkg/config"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPublisher_Init(t *testing.T) {
	k0sVars, err := config.NewCfgVars(nil, t.TempDir())
	require.NoError(t, err)
	require.NoError(t, os.MkdirAll(k0sVars.CertRootDir, 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(k0sVars.CertRootDir, "ca.crt"), []byte("made-up CA"), 0o600))
	nodeConfig := v1beta1.DefaultClusterConfig()
	nodeConfig.Spec.API.Address = "192.0.2.1"
	clients := testutil.NewFakeClientFactory()
	getClusterInfo := func() error {
		_, err := clients.Client.CoreV1().ConfigMaps("kube-public").Get(t.Context(), "cluster-info", metav1.GetOptions{})
		return err
	}

	nodeConfig.Spec.API.ClusterInfoDiscovery = true
	require.NoError(t, (&Publisher{K0sVars: k0sVars, NodeConfig: nodeConfig, Clients: clients}).Init(t.Context()))
	assert.NoError(t, getClusterInfo(), "the cluster-info should be published while enabled")

	nodeConfig.Spec.API.ClusterInfoDiscovery = false
	require.NoError(t, (&Publisher{K0sVars: k0sVars, NodeConfig: nodeConfig, Clients: clients}).Init(t.Context()))
	assert.True(t, apierrors.IsNotFound(getClusterInfo()), "the cluster-info should be removed once disabled")
}
