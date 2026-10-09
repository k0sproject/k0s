// SPDX-FileCopyrightText: 2026 k0s authors
// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"crypto/x509"
	"encoding/pem"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/k0sproject/k0s/pkg/apis/k0s/v1beta1"
	"github.com/k0sproject/k0s/pkg/certificate"
	"github.com/k0sproject/k0s/pkg/config"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The API server certificate needs to be accepted for the in-cluster service
// name, no matter if clients spell it as a relative or as a fully qualified
// (trailing dot) domain name. A separate SAN for the fully qualified spelling
// isn't required for that: hostname verification strips the trailing dot from
// the name that's being verified.
func TestGenerateSANList_ClusterDomainNames(t *testing.T) {
	clusterSpec := v1beta1.DefaultClusterSpec()
	serverCert, verifyOpts := issueServerCert(t, clusterSpec)

	for _, hostname := range []string{
		"kubernetes",
		"kubernetes.default",
		"kubernetes.default.svc",
		"kubernetes.default.svc." + clusterSpec.Network.ClusterDomain,
		"kubernetes.default.svc." + clusterSpec.Network.ClusterDomain + ".",
		"localhost",
	} {
		t.Run(hostname, func(t *testing.T) {
			opts := verifyOpts
			opts.DNSName = hostname
			_, err := serverCert.Verify(opts)
			assert.NoError(t, err)
		})
	}
}

// issueServerCert generates the API server's SAN list for the given cluster
// spec and issues the server certificate for it, just as k0s does at runtime.
// It returns the parsed certificate along with the verification options that
// validate it against the CA that issued it.
func issueServerCert(t *testing.T, clusterSpec *v1beta1.ClusterSpec) (*x509.Certificate, x509.VerifyOptions) {
	k0sVars, err := config.NewCfgVars(nil, t.TempDir())
	require.NoError(t, err)
	require.NoError(t, os.MkdirAll(k0sVars.CertRootDir, 0755))

	certManager := certificate.Manager{K0sVars: k0sVars}
	require.NoError(t, certManager.EnsureCA("ca", "kubernetes-ca", time.Hour))

	underTest := Certificates{
		CertManager: certManager,
		ClusterSpec: clusterSpec,
		K0sVars:     k0sVars,
	}

	hostnames, err := underTest.generateSANList(t.Context())
	require.NoError(t, err)

	serverCert, err := certManager.EnsureCertificate(certificate.Request{
		Name:      "server",
		CN:        "kubernetes",
		O:         "kubernetes",
		CACert:    filepath.Join(k0sVars.CertRootDir, "ca.crt"),
		CAKey:     filepath.Join(k0sVars.CertRootDir, "ca.key"),
		Hostnames: hostnames,
	}, os.Getuid(), time.Hour)
	require.NoError(t, err)

	caCert, err := os.ReadFile(filepath.Join(k0sVars.CertRootDir, "ca.crt"))
	require.NoError(t, err)
	roots := x509.NewCertPool()
	require.True(t, roots.AppendCertsFromPEM(caCert), "Failed to load CA certificate")

	return parseCert(t, []byte(serverCert.Cert)), x509.VerifyOptions{
		Roots:     roots,
		KeyUsages: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}
}

func parseCert(t *testing.T, pemBytes []byte) *x509.Certificate {
	block, _ := pem.Decode(pemBytes)
	require.NotNil(t, block, "Failed to decode PEM data")
	cert, err := x509.ParseCertificate(block.Bytes)
	require.NoError(t, err)
	return cert
}
