// SPDX-FileCopyrightText: 2025 k0s authors
// SPDX-License-Identifier: Apache-2.0

package certificate

import (
	"crypto/x509"
	"encoding/pem"
	"os"
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

func parseCert(pemBytes []byte) (*x509.Certificate, error) {
	block, _ := pem.Decode(pemBytes)
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return nil, err
	}
	return cert, nil
}
