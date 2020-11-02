/*
Copyright 2020 Mirantis, Inc.

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
package certificate

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/cloudflare/cfssl/certinfo"
	"github.com/cloudflare/cfssl/cli"
	"github.com/cloudflare/cfssl/cli/genkey"
	"github.com/cloudflare/cfssl/cli/sign"
	"github.com/cloudflare/cfssl/csr"
	"github.com/cloudflare/cfssl/initca"
	"github.com/cloudflare/cfssl/signer"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"

	"github.com/Mirantis/mke/pkg/constant"
	"github.com/Mirantis/mke/pkg/util"
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
}

// EnsureCA makes sure the given CA certs and key is created.
func (m *Manager) EnsureCA(name, cn string) error {
	keyFile := filepath.Join(constant.CertRootDir, fmt.Sprintf("%s.key", name))
	certFile := filepath.Join(constant.CertRootDir, fmt.Sprintf("%s.crt", name))

	if util.FileExists(keyFile) && util.FileExists(certFile) {
		return nil
	}

	req := new(csr.CertificateRequest)
	req.KeyRequest = csr.NewKeyRequest()
	req.KeyRequest.A = "rsa"
	req.KeyRequest.S = 2048
	req.CN = cn
	req.CA = &csr.CAConfig{
		Expiry: "87600h",
	}
	cert, _, key, err := initca.New(req)
	if err != nil {
		return err
	}

	err = ioutil.WriteFile(keyFile, key, constant.CertSecureMode)
	if err != nil {
		return err
	}

	err = ioutil.WriteFile(certFile, cert, constant.CertMode)
	if err != nil {
		return err
	}

	gid, err := util.GetGID(constant.Group)
	if err == nil {
		paths := []string{filepath.Dir(keyFile), keyFile, certFile}
		for _, path := range paths {
			err = os.Chown(path, -1, gid)
			if err != nil {
				logrus.Warning(err)
			}
		}
	} else {
		logrus.Warning(err)
	}

	return nil
}

// EnsureCertificate creates the specified certificate if it does not already exist
func (m *Manager) EnsureCertificate(certReq Request, ownerName string) (Certificate, error) {

	keyFile := filepath.Join(constant.CertRootDir, fmt.Sprintf("%s.key", certReq.Name))
	certFile := filepath.Join(constant.CertRootDir, fmt.Sprintf("%s.crt", certReq.Name))

	gid, _ := util.GetGID(constant.Group)
	uid, _ := util.GetUID(ownerName)

	if util.FileExists(keyFile) && util.FileExists(certFile) {
		err := m.EnsureSANHosts(certReq, certFile)
		if err != nil {
			return Certificate{}, err
		}

		_ = os.Chown(keyFile, uid, gid)
		_ = os.Chown(certFile, uid, gid)

		cert, err := ioutil.ReadFile(certFile)
		if err != nil {
			return Certificate{}, errors.Wrapf(err, "failed to read ca cert %s for %s", certFile, certReq.Name)
		}
		key, err := ioutil.ReadFile(keyFile)
		if err != nil {
			return Certificate{}, errors.Wrapf(err, "failed to read ca key %s for %s", keyFile, certReq.Name)
		}

		return Certificate{
			Key:  string(key),
			Cert: string(cert),
		}, nil
	}

	req := csr.CertificateRequest{
		KeyRequest: csr.NewKeyRequest(),
		CN:         certReq.CN,
		Names: []csr.Name{
			{O: certReq.O},
		},
	}

	req.KeyRequest.A = "rsa"
	req.KeyRequest.S = 2048
	req.Hosts = certReq.Hostnames

	var key, csrBytes []byte
	g := &csr.Generator{Validator: genkey.Validator}
	csrBytes, key, err := g.ProcessRequest(&req)
	if err != nil {
		return Certificate{}, err
	}
	config := cli.Config{
		CAFile:    certReq.CACert,
		CAKeyFile: certReq.CAKey,
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
	err = ioutil.WriteFile(keyFile, key, constant.CertSecureMode)
	if err != nil {
		return Certificate{}, err
	}
	err = ioutil.WriteFile(certFile, cert, constant.CertMode)
	if err != nil {
		return Certificate{}, err
	}

	err = os.Chown(keyFile, uid, gid)
	if err != nil {
		return Certificate{}, err
	}
	err = os.Chown(certFile, uid, gid)
	if err != nil {
		return Certificate{}, err
	}

	return c, nil
}

// EnsureSANHosts verifies if the certificates contains the correct SANs
// If not, it removes the certificate and forces generation of a new Cert
func (m *Manager) EnsureSANHosts(certReq Request, certFile string) (err error) {
	var cert *certinfo.Certificate

	if cert, err = certinfo.ParseCertificateFile(certFile); err != nil {
		return fmt.Errorf("unable to parse certificate file: %v", err)
	}

	// if existing SANs are different than configured, delete the certificate to re-generat it
	if !util.IsStringArrayEqual(certReq.Hostnames, cert.SANs) {
		logrus.Debug("found changes in SAN configuration. attempting to re-generate files")
		// find and remove files to remove
		filter := fmt.Sprintf("%s.*", strings.TrimSuffix(certFile, filepath.Ext(certFile))) // returns <cert_name>.*
		files, err := filepath.Glob(filter)
		if err != nil {
			logrus.Errorf("unable to find files to clean up: %v", err)
		}
		for _, f := range files {
			logrus.Debugf("deleting file %v for certificate re-generation", f)
			if err := os.Remove(f); err != nil {
				logrus.Errorf("failed to delete file: %v", f)
			}
		}
	}
	return nil
}
