// SPDX-FileCopyrightText: 2020 k0s authors
// SPDX-License-Identifier: Apache-2.0

package certificate

import (
	"bufio"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/cloudflare/cfssl/certinfo"
	"github.com/cloudflare/cfssl/cli"
	"github.com/cloudflare/cfssl/cli/genkey"
	"github.com/cloudflare/cfssl/cli/sign"
	cfsslconfig "github.com/cloudflare/cfssl/config"
	"github.com/cloudflare/cfssl/csr"
	"github.com/cloudflare/cfssl/initca"
	"github.com/cloudflare/cfssl/signer"
	"github.com/sirupsen/logrus"

	"github.com/k0sproject/k0s/internal/pkg/file"
	"github.com/k0sproject/k0s/internal/pkg/stringslice"
	"github.com/k0sproject/k0s/pkg/config"
	"github.com/k0sproject/k0s/pkg/constant"
)

// Request defines the certificate request fields
type Request struct {
	Name      string
	CN        string
	O         string
	CAKey     string
	CACert    string
	Hostnames []string
}

// Certificate is a helper struct to be able to return the created key and cert data
type Certificate struct {
	Key  string
	Cert string
}

// Manager is the certificate manager
type Manager struct {
	K0sVars *config.CfgVars
}

// EnsureCA makes sure the given CA certs and key is created.
func (m *Manager) EnsureCA(name, cn string, expiry time.Duration) error {
	keyFile := filepath.Join(m.K0sVars.CertRootDir, name+".key")
	certFile := filepath.Join(m.K0sVars.CertRootDir, name+".crt")

	if file.Exists(keyFile) && file.Exists(certFile) {
		return nil
	}

	req := new(csr.CertificateRequest)
	req.KeyRequest = csr.NewKeyRequest()
	req.KeyRequest.A = "rsa"
	req.KeyRequest.S = 2048
	req.CN = cn
	req.CA = &csr.CAConfig{
		Expiry: expiry.String(),
	}
	cert, _, key, err := initca.New(req)
	if err != nil {
		return err
	}

	err = file.WriteContentAtomically(keyFile, key, constant.CertSecureMode)
	if err != nil {
		return err
	}

	err = file.WriteContentAtomically(certFile, cert, constant.CertMode)
	if err != nil {
		return err
	}

	return nil
}

// EnsureCertificate creates the specified certificate if it does not already exist
func (m *Manager) EnsureCertificate(certReq Request, ownerID int, expiry time.Duration) (Certificate, error) {

	keyFile := filepath.Join(m.K0sVars.CertRootDir, certReq.Name+".key")
	certFile := filepath.Join(m.K0sVars.CertRootDir, certReq.Name+".crt")

	// if regenerateCert returns true, it means we need to create the certs
	regenerateCert, err := m.regenerateCert(keyFile, certFile)
	if err != nil {
		return Certificate{}, err
	}
	if regenerateCert {
		logrus.Debugf("creating certificate %s", certFile)
		req := csr.CertificateRequest{
			KeyRequest: csr.NewKeyRequest(),
			CN:         certReq.CN,
			Names: []csr.Name{
				{O: certReq.O},
			},
		}

		req.KeyRequest.A = "rsa"
		req.KeyRequest.S = 2048
		req.Hosts = stringslice.Unique(certReq.Hostnames)

		var key, csrBytes []byte
		g := &csr.Generator{Validator: genkey.Validator}
		csrBytes, key, err := g.ProcessRequest(&req)
		if err != nil {
			return Certificate{}, err
		}
		config := cli.Config{
			CAFile:    "file:" + certReq.CACert,
			CAKeyFile: "file:" + certReq.CAKey,
			CFG: &cfsslconfig.Config{
				Signing: &cfsslconfig.Signing{
					Profiles: map[string]*cfsslconfig.SigningProfile{},
					Default: &cfsslconfig.SigningProfile{
						Usage:        []string{"signing", "key encipherment", "server auth", "client auth"},
						Expiry:       expiry,
						ExpiryString: expiry.String(),
					},
				},
			},
		}
		s, err := sign.SignerFromConfig(config)
		if err != nil {
			return Certificate{}, err
		}

		var cert []byte
		signReq := signer.SignRequest{
			Request: string(csrBytes),
			Profile: "kubernetes",
		}

		cert, err = s.Sign(signReq)
		if err != nil {
			return Certificate{}, err
		}
		c := Certificate{
			Key:  string(key),
			Cert: string(cert),
		}
		err = file.WriteContentAtomically(keyFile, key, constant.CertSecureMode)
		if err != nil {
			return Certificate{}, err
		}
		err = file.WriteContentAtomically(certFile, cert, constant.CertMode)
		if err != nil {
			return Certificate{}, err
		}

		err = os.Chown(keyFile, ownerID, -1)
		if err != nil && os.Geteuid() == 0 {
			return Certificate{}, err
		}
		err = os.Chown(certFile, ownerID, -1)
		if err != nil && os.Geteuid() == 0 {
			return Certificate{}, err
		}

		return c, nil
	}

	// certs exist, let's just verify their permissions
	_ = os.Chown(keyFile, ownerID, -1)
	_ = os.Chown(certFile, ownerID, -1)

	cert, err := os.ReadFile(certFile)
	if err != nil {
		return Certificate{}, fmt.Errorf("failed to read ca cert %s for %s: %w", certFile, certReq.Name, err)
	}
	key, err := os.ReadFile(keyFile)
	if err != nil {
		return Certificate{}, fmt.Errorf("failed to read ca key %s for %s: %w", keyFile, certReq.Name, err)
	}

	return Certificate{
		Key:  string(key),
		Cert: string(cert),
	}, nil

}

// if regenerateCert does not need to do any changes, it will return false
// if a change in SAN hosts is detected, if will return true, to re-generate certs
func (m *Manager) regenerateCert(keyFile string, certFile string) (bool, error) {
	var cert *certinfo.Certificate
	var err error

	// if certificate & key don't exist, return true, in order to generate certificates
	if !file.Exists(keyFile) && !file.Exists(certFile) {
		return true, nil
	}

	if cert, err = certinfo.ParseCertificateFile(certFile); err != nil {
		logrus.Warnf("unable to parse certificate file at %s: %v", certFile, err)
		return true, nil
	}

	if managed, err := m.isManagedByK0s(cert); err != nil || managed {
		return managed, err
	}

	logrus.Debugf("cert regeneration not needed for %s, not managed by k0s: %s", certFile, cert.Issuer.CommonName)
	return false, nil
}

// checks if the cert issuer (CA) is a k0s setup one
func (m *Manager) isManagedByK0s(cert *certinfo.Certificate) (bool, error) {
	ca, err := certinfo.ParseCertificateFile(filepath.Join(m.K0sVars.CertRootDir, "ca.crt"))
	if err != nil {
		return false, fmt.Errorf("unable to parse ca certificate: %w", err)
	}

	switch cert.Issuer.CommonName {
	case "kubernetes-ca":
		return true, nil
	case "kubernetes-front-proxy-ca":
		return true, nil
	case "etcd-ca":
		return true, nil
	case ca.Subject.CommonName:
		if file.Exists(filepath.Join(m.K0sVars.CertRootDir, "ca.key")) {
			return true, nil
		}
		logrus.Warnf("certificate issued by %q, but no ca.key found, not renewing the certificate %q", ca.Subject.CommonName, cert.Subject.CommonName)
		return false, nil
	}

	return false, nil
}

func (m *Manager) CreateKeyPair(name string, k0sVars *config.CfgVars, ownerID int) error {
	keyFile := filepath.Join(k0sVars.CertRootDir, name+".key")
	pubFile := filepath.Join(k0sVars.CertRootDir, name+".pub")

	if file.Exists(keyFile) && file.Exists(pubFile) {
		return file.Chown(keyFile, ownerID, constant.CertSecureMode)
	}

	reader := rand.Reader
	bitSize := 2048

	key, err := rsa.GenerateKey(reader, bitSize)
	if err != nil {
		return err
	}

	err = file.WriteAtomically(keyFile, constant.CertSecureMode, func(unbuffered io.Writer) error {
		privateKey := pem.Block{
			Type:  "RSA PRIVATE KEY",
			Bytes: x509.MarshalPKCS1PrivateKey(key),
		}

		w := bufio.NewWriter(unbuffered)
		if err := pem.Encode(w, &privateKey); err != nil {
			return err
		}
		return w.Flush()
	})

	if err != nil {
		return err
	}

	return file.WriteAtomically(pubFile, 0644, func(unbuffered io.Writer) error {
		// note to the next reader: key.Public() != key.PublicKey
		pubBytes, err := x509.MarshalPKIXPublicKey(key.Public())
		if err != nil {
			return err
		}

		pemKey := pem.Block{
			Type:  "PUBLIC KEY",
			Bytes: pubBytes,
		}

		w := bufio.NewWriter(unbuffered)
		if err := pem.Encode(w, &pemKey); err != nil {
			return err
		}
		return w.Flush()
	})
}
