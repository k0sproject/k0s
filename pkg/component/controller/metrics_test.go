// SPDX-FileCopyrightText: 2026 k0s authors
// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/pem"
	"math/big"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/k0sproject/k0s/pkg/apis/k0s/v1beta1"
	"github.com/k0sproject/k0s/pkg/config"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
)

func TestEtcdMetricsJobTarget(t *testing.T) {
	dir := t.TempDir()
	writeEtcdClientCert(t, dir)
	vars := &config.CfgVars{
		CertRootDir:    dir,
		EtcdSocketPath: "/run/k0s/etcd/etcd.sock",
	}

	managed, err := (&Metrics{K0sVars: vars, log: logrus.New()}).newEtcdJob()
	require.NoError(t, err)
	require.Equal(t, "https://localhost/metrics", managed.scrapeURL)
	require.ErrorContains(t, dial(t, managed), vars.EtcdSocketPath)

	external, err := (&Metrics{
		K0sVars: vars,
		log:     logrus.New(),
		etcd: &v1beta1.EtcdConfig{ExternalCluster: &v1beta1.ExternalCluster{
			Endpoints: []string{"https://10.0.0.1:2379"},
		}},
	}).newEtcdJob()
	require.NoError(t, err)
	require.Equal(t, "https://localhost:2379/metrics", external.scrapeURL)
	require.ErrorContains(t, dial(t, external), "127.0.0.1:1")
}

func dial(t *testing.T, j *job) error {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 50*time.Millisecond)
	defer cancel()
	_, err := j.scrapeClient.Transport.(*http.Transport).DialContext(ctx, "tcp", "127.0.0.1:1")
	return err
}

func writeEtcdClientCert(t *testing.T, dir string) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	template := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		NotBefore:    time.Now(),
		NotAfter:     time.Now().Add(time.Hour),
	}
	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	require.NoError(t, err)
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	keyDER, err := x509.MarshalECPrivateKey(key)
	require.NoError(t, err)
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER})
	require.NoError(t, os.WriteFile(filepath.Join(dir, "apiserver-etcd-client.crt"), certPEM, 0600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "apiserver-etcd-client.key"), keyPEM, 0600))
}
