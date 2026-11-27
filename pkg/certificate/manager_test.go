// SPDX-FileCopyrightText: 2025 k0s authors
// SPDX-License-Identifier: Apache-2.0

package certificate

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	cfsslconfig "github.com/cloudflare/cfssl/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/k0sproject/k0s/pkg/config"
)

func TestEnsureCA(t *testing.T) {
	// Create some k0sVars
	k0sVars, err := config.NewCfgVars(nil, t.TempDir())
	require.NoError(t, err)

	// Create the CA
	require.NoError(t, os.MkdirAll(k0sVars.CertRootDir, 0755))
	certManager := Manager{K0sVars: k0sVars}
	require.NoError(t, certManager.EnsureCA("ca", t.Name(), 100000*time.Hour))

	pemBytes, _ := os.ReadFile(k0sVars.CertRootDir + "/ca.crt")
	cert, err := parseCert(pemBytes)
	require.NoError(t, err)
	// check the expiration date of the cert
	assert.Equal(t, cert.NotBefore.Add(100000*time.Hour), cert.NotAfter)
}

func TestEnsureCertificate(t *testing.T) {
	// Create some k0sVars
	k0sVars, err := config.NewCfgVars(nil, t.TempDir())
	require.NoError(t, err)

	// Create the CA
	require.NoError(t, os.MkdirAll(k0sVars.CertRootDir, 0755))
	certManager := Manager{K0sVars: k0sVars}
	require.NoError(t, certManager.EnsureCA("ca", t.Name(), 100000*time.Hour))

	req := Request{
		Name:   "test",
		CN:     "kubernetes-test",
		O:      "system:masters",
		CACert: k0sVars.CertRootDir + "/ca.crt",
		CAKey:  k0sVars.CertRootDir + "/ca.key",
	}
	certData, err := certManager.EnsureCertificate(req, 1, 10000*time.Hour)
	require.NoError(t, err)
	cert, err := parseCert([]byte(certData.Cert))
	require.NoError(t, err)

	// check the expiration date of the cert
	assert.Equal(t, cert.NotBefore.Add(10000*time.Hour), cert.NotAfter)
	// check if the cert has the `signing` usage
	assert.NotEqual(t, 0, cert.KeyUsage&x509.KeyUsageDigitalSignature)
	// check if the cert has the `key encipherment` usage
	assert.NotEqual(t, 0, cert.KeyUsage&x509.KeyUsageKeyEncipherment)
	assert.Equal(t,
		[]x509.ExtKeyUsage{cfsslconfig.ExtKeyUsage["server auth"], cfsslconfig.ExtKeyUsage["client auth"]},
		cert.ExtKeyUsage,
	)
}

func TestExpiredCertificatesAreRejected(t *testing.T) {
	writeCert := func(t *testing.T, path string, ca *x509.Certificate, caKey *rsa.PrivateKey, cn string, notAfter time.Time) (*x509.Certificate, *rsa.PrivateKey) {
		key, err := rsa.GenerateKey(rand.Reader, 2048)
		require.NoError(t, err)
		tmpl := &x509.Certificate{
			SerialNumber:          big.NewInt(time.Now().UnixNano()),
			Subject:               pkix.Name{CommonName: cn},
			NotBefore:             notAfter.Add(-48 * time.Hour),
			NotAfter:              notAfter,
			IsCA:                  ca == nil,
			BasicConstraintsValid: true,
		}
		parent, signer := tmpl, key
		if ca != nil {
			parent, signer = ca, caKey
		}
		der, err := x509.CreateCertificate(rand.Reader, tmpl, parent, &key.PublicKey, signer)
		require.NoError(t, err)
		require.NoError(t, os.WriteFile(path, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), 0644))
		keyPath := strings.TrimSuffix(path, ".crt") + ".key"
		require.NoError(t, os.WriteFile(keyPath, pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)}), 0600))
		parsed, err := x509.ParseCertificate(der)
		require.NoError(t, err)
		return parsed, key
	}

	past, future := time.Now().Add(-time.Hour), time.Now().Add(24*time.Hour)

	t.Run("EnsureCA", func(t *testing.T) {
		for _, test := range []struct {
			name     string
			notAfter time.Time
			expired  bool
		}{
			{"expired", past, true},
			{"valid", future, false},
		} {
			t.Run(test.name, func(t *testing.T) {
				k0sVars, err := config.NewCfgVars(nil, t.TempDir())
				require.NoError(t, err)
				require.NoError(t, os.MkdirAll(k0sVars.CertRootDir, 0755))
				caFile := filepath.Join(k0sVars.CertRootDir, "ca.crt")
				writeCert(t, caFile, nil, nil, "kubernetes-ca", test.notAfter)

				err = (&Manager{K0sVars: k0sVars}).EnsureCA("ca", "kubernetes-ca", time.Hour)
				if test.expired {
					require.Error(t, err)
					assert.Contains(t, err.Error(), caFile)
					assert.Contains(t, err.Error(), "expired")
				} else {
					assert.NoError(t, err)
				}
			})
		}
	})

	t.Run("EnsureCertificate", func(t *testing.T) {
		for _, test := range []struct {
			name     string
			notAfter time.Time
			expired  bool
		}{
			{"expired user-provided", past, true},
			{"valid user-provided", future, false},
		} {
			t.Run(test.name, func(t *testing.T) {
				k0sVars, err := config.NewCfgVars(nil, t.TempDir())
				require.NoError(t, err)
				require.NoError(t, os.MkdirAll(k0sVars.CertRootDir, 0755))
				writeCert(t, filepath.Join(k0sVars.CertRootDir, "ca.crt"), nil, nil, "kubernetes-ca", future)
				// The cert is issued by some external CA, so k0s doesn't manage it.
				ca, caKey := writeCert(t, filepath.Join(t.TempDir(), "external.crt"), nil, nil, "external-ca", future)
				certFile := filepath.Join(k0sVars.CertRootDir, "custom.crt")
				writeCert(t, certFile, ca, caKey, "custom", test.notAfter)

				req := Request{Name: "custom", CN: "custom", CACert: filepath.Join(k0sVars.CertRootDir, "ca.crt"), CAKey: filepath.Join(k0sVars.CertRootDir, "ca.key")}
				_, err = (&Manager{K0sVars: k0sVars}).EnsureCertificate(req, -1, time.Hour)
				if test.expired {
					require.Error(t, err)
					assert.Contains(t, err.Error(), certFile)
					assert.Contains(t, err.Error(), "expired")
				} else {
					assert.NoError(t, err)
				}
			})
		}
	})
}

func parseCert(pemBytes []byte) (*x509.Certificate, error) {
	block, _ := pem.Decode(pemBytes)
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return nil, err
	}
	return cert, nil
}
