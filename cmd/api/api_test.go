// SPDX-FileCopyrightText: 2025 k0s authors
// SPDX-License-Identifier: Apache-2.0

package api_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/iotest"
	"time"

	"github.com/cloudflare/cfssl/csr"
	"github.com/cloudflare/cfssl/initca"
	"github.com/k0sproject/k0s/cmd"
	"github.com/k0sproject/k0s/pkg/apis/k0s/v1beta1"
	"github.com/k0sproject/k0s/pkg/config"
	"github.com/k0sproject/k0s/pkg/k0scontext"
	"github.com/sirupsen/logrus"
	"github.com/sirupsen/logrus/hooks/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/yaml"

	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
)

func TestAPI(t *testing.T) {
	t.Run("MissingRuntimeConfig", func(t *testing.T) {
		underTest := cmd.NewRootCmd()
		underTest.SetArgs([]string{"api"})
		underTest.SetIn(iotest.ErrReader(io.EOF))
		err := underTest.ExecuteContext(t.Context())
		assert.ErrorContains(t, err, `failed to load runtime configuration: invalid runtime configuration: invalid api version: ""`)
	})

	dataDir := t.TempDir()
	rtc := config.RuntimeConfig{
		TypeMeta: v1.TypeMeta{APIVersion: v1beta1.ClusterConfigAPIVersion, Kind: config.RuntimeConfigKind},
		Spec: &config.RuntimeConfigSpec{
			NodeConfig: &v1beta1.ClusterConfig{Spec: &v1beta1.ClusterSpec{
				API: &v1beta1.APISpec{
					Address:           "127.0.0.1",
					OnlyBindToAddress: true,
				},
				Storage: &v1beta1.StorageSpec{},
			}},
			K0sVars: &config.CfgVars{
				AdminKubeConfigPath: filepath.Join(dataDir, "kubeconfig"),
				CertRootDir:         dataDir,
			},
		},
	}
	// Find a free port. We cannot pass zero to the API since this will fallback to 9443.
	if l, err := net.Listen("tcp", "127.0.0.1:0"); assert.NoError(t, err) {
		// Extract the port number
		addr := l.Addr().(*net.TCPAddr)
		rtc.Spec.NodeConfig.Spec.API.K0sAPIPort = addr.Port
		require.NoError(t, l.Close())
	} else {
		rtc.Spec.NodeConfig.Spec.API.K0sAPIPort = 9443
	}

	configData, err := yaml.Marshal(&rtc)
	require.NoError(t, err)

	t.Run("MissingKubeconfig", func(t *testing.T) {
		underTest := cmd.NewRootCmd()
		underTest.SetArgs([]string{"api"})
		underTest.SetIn(bytes.NewReader(configData))
		err := underTest.ExecuteContext(t.Context())
		var pathErr *os.PathError
		if assert.ErrorAs(t, err, &pathErr) {
			assert.Equal(t, pathErr.Path, rtc.Spec.K0sVars.AdminKubeConfigPath)
			assert.ErrorIs(t, pathErr.Err, os.ErrNotExist)
		}
	})

	kubeconfig := clientcmdapi.Config{
		Clusters:       map[string]*clientcmdapi.Cluster{t.Name(): {Server: "blackhole.example.com"}},
		Contexts:       map[string]*clientcmdapi.Context{t.Name(): {Cluster: t.Name()}},
		CurrentContext: t.Name(),
	}
	require.NoError(t, clientcmd.WriteToFile(kubeconfig, rtc.Spec.K0sVars.AdminKubeConfigPath))

	t.Run("MissingCertificate", func(t *testing.T) {
		underTest := cmd.NewRootCmd()
		underTest.SetArgs([]string{"api"})
		underTest.SetIn(bytes.NewReader(configData))
		err := underTest.ExecuteContext(t.Context())
		var pathErr *os.PathError
		if assert.ErrorAs(t, err, &pathErr) {
			assert.Equal(t, pathErr.Path, filepath.Join(rtc.Spec.K0sVars.CertRootDir, "k0s-api.crt"))
			assert.ErrorIs(t, pathErr.Err, os.ErrNotExist)
		}
	})

	certData, _, keyData, err := initca.New(&csr.CertificateRequest{
		KeyRequest: csr.NewKeyRequest(),
		CN:         "blackhole.example.com",
	})
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(rtc.Spec.K0sVars.CertRootDir, "k0s-api.crt"), certData, 0644))
	require.NoError(t, os.WriteFile(filepath.Join(rtc.Spec.K0sVars.CertRootDir, "k0s-api.key"), keyData, 0600))

	t.Run("StartsAndStops", func(t *testing.T) {
		ctx, cancel := context.WithCancelCause(t.Context())
		defer cancel(errors.New("test function exited"))

		var logsConsumed uint
		log, allLogs := test.NewNullLogger()
		ctx = k0scontext.WithValue[logrus.FieldLogger](ctx, log)

		underTest := cmd.NewRootCmd()
		underTest.SetArgs([]string{"api"})
		underTest.SetIn(bytes.NewReader(configData))

		errCh := make(chan error, 1)
		go func() { errCh <- underTest.ExecuteContext(ctx) }()

	startup:
		for {
			select {
			case err := <-errCh:
				require.Failf(t, "API terminated unexpectedly", "%v", err)

			case <-time.After(100 * time.Millisecond):
				for _, entry := range allLogs.AllEntries()[logsConsumed:] {
					t.Log(entry.Message)
					logsConsumed++
					if entry.Message == fmt.Sprintf(
						"Listening on %s:%d, start serving",
						rtc.Spec.NodeConfig.Spec.API.Address,
						rtc.Spec.NodeConfig.Spec.API.K0sAPIPort,
					) {
						cancel(errors.New(t.Name() + " succeeded"))
						break startup
					}
				}
			}
		}

		assert.NoError(t, <-errCh, "API didn't terminate successfully")
		var shutdownReasonFound bool
		for _, entry := range allLogs.AllEntries()[logsConsumed:] {
			t.Log(entry.Message)
			if !shutdownReasonFound {
				if reason, found := strings.CutPrefix(entry.Message, "Shutting down server: "); found {
					shutdownReasonFound = true
					assert.Equal(t, t.Name()+" succeeded", reason, "Unexpected shutdown reason")
				}
			}
		}

		assert.True(t, shutdownReasonFound, "No shutdown reason found in API logs")
	})
}
