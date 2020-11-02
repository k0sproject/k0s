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
package server

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"text/template"

	"golang.org/x/sync/errgroup"

	config "github.com/Mirantis/mke/pkg/apis/v1beta1"
	"github.com/Mirantis/mke/pkg/certificate"
	"github.com/Mirantis/mke/pkg/constant"
	"github.com/Mirantis/mke/pkg/util"
	"github.com/pkg/errors"

	"github.com/sirupsen/logrus"
)

var (
	kubeconfigTemplate = template.Must(template.New("kubeconfig").Parse(`
apiVersion: v1
clusters:
- cluster:
    server: {{.URL}}
    certificate-authority-data: {{.CACert}}
  name: local
contexts:
- context:
    cluster: local
    namespace: default
    user: user
  name: Default
current-context: Default
kind: Config
preferences: {}
users:
- name: user
  user:
    client-certificate-data: {{.ClientCert}}
    client-key-data: {{.ClientKey}}
`))
)

// Certificates is the Component implementation to manage all mke certs
type Certificates struct {
	CACert string

	CertManager certificate.Manager
	ClusterSpec *config.ClusterSpec
}

// Init initializes the certificate component
func (c *Certificates) Init() error {

	eg, _ := errgroup.WithContext(context.Background())
	// Common CA

	caCertPath, caCertKey := filepath.Join(constant.CertRootDir, "ca.crt"), filepath.Join(constant.CertRootDir, "ca.key")

	if err := c.CertManager.EnsureCA("ca", "kubernetes-ca"); err != nil {
		return err
	}

	// We need CA cert loaded to generate client configs
	logrus.Debugf("CA key and cert exists, loading")
	cert, err := ioutil.ReadFile(caCertPath)
	if err != nil {
		return errors.Wrapf(err, "failed to read ca cert")
	}
	c.CACert = string(cert)

	eg.Go(func() error {
		// Front proxy CA
		if err := c.CertManager.EnsureCA("front-proxy-ca", "kubernetes-front-proxy-ca"); err != nil {
			return err
		}

		proxyCertPath, proxyCertKey := filepath.Join(constant.CertRootDir, "front-proxy-ca.crt"), filepath.Join(constant.CertRootDir, "front-proxy-ca.key")

		proxyClientReq := certificate.Request{
			Name:   "front-proxy-client",
			CN:     "front-proxy-client",
			O:      "front-proxy-client",
			CACert: proxyCertPath,
			CAKey:  proxyCertKey,
		}
		_, err := c.CertManager.EnsureCertificate(proxyClientReq, constant.ApiserverUser)

		return err
	})

	eg.Go(func() error {
		// admin cert & kubeconfig
		adminReq := certificate.Request{
			Name:   "admin",
			CN:     "kubernetes-admin",
			O:      "system:masters",
			CACert: caCertPath,
			CAKey:  caCertKey,
		}
		adminCert, err := c.CertManager.EnsureCertificate(adminReq, "root")
		if err != nil {
			return err
		}
		if err := kubeConfig(constant.AdminKubeconfigConfigPath, "https://localhost:6443", c.CACert, adminCert.Cert, adminCert.Key); err != nil {
			return err
		}

		return generateKeyPair("sa")
	})

	eg.Go(func() error {
		ccmReq := certificate.Request{
			Name:   "ccm",
			CN:     "system:kube-controller-manager",
			O:      "system:kube-controller-manager",
			CACert: caCertPath,
			CAKey:  caCertKey,
		}
		ccmCert, err := c.CertManager.EnsureCertificate(ccmReq, constant.ControllerManagerUser)

		if err != nil {
			return err
		}

		return kubeConfig(filepath.Join(constant.CertRootDir, "ccm.conf"), "https://localhost:6443", c.CACert, ccmCert.Cert, ccmCert.Key)
	})

	eg.Go(func() error {
		schedulerReq := certificate.Request{
			Name:   "scheduler",
			CN:     "system:kube-scheduler",
			O:      "system:kube-scheduler",
			CACert: caCertPath,
			CAKey:  caCertKey,
		}
		schedulerCert, err := c.CertManager.EnsureCertificate(schedulerReq, constant.SchedulerUser)
		if err != nil {
			return err
		}

		return kubeConfig(filepath.Join(constant.CertRootDir, "scheduler.conf"), "https://localhost:6443", c.CACert, schedulerCert.Cert, schedulerCert.Key)
	})

	eg.Go(func() error {
		kubeletClientReq := certificate.Request{
			Name:   "apiserver-kubelet-client",
			CN:     "apiserver-kubelet-client",
			O:      "system:masters",
			CACert: caCertPath,
			CAKey:  caCertKey,
		}
		_, err := c.CertManager.EnsureCertificate(kubeletClientReq, constant.ApiserverUser)
		return err
	})

	hostnames := []string{
		"kubernetes",
		"kubernetes.default",
		"kubernetes.default.svc",
		"kubernetes.default.svc.cluster",
		"kubernetes.svc.cluster.local",
		"127.0.0.1",
		"localhost",
	}

	hostnames = append(hostnames, c.ClusterSpec.API.Address)
	hostnames = append(hostnames, c.ClusterSpec.API.SANs...)

	internalAPIAddress, err := c.ClusterSpec.Network.InternalAPIAddress()
	if err != nil {
		return err
	}
	hostnames = append(hostnames, internalAPIAddress)

	eg.Go(func() error {
		serverReq := certificate.Request{
			Name:      "server",
			CN:        "kubernetes",
			O:         "kubernetes",
			CACert:    caCertPath,
			CAKey:     caCertKey,
			Hostnames: hostnames,
		}
		_, err = c.CertManager.EnsureCertificate(serverReq, constant.ApiserverUser)

		return err
	})

	eg.Go(func() error {
		mkeAPIReq := certificate.Request{
			Name:      "mke-api",
			CN:        "mke-api",
			O:         "kubernetes",
			CACert:    caCertPath,
			CAKey:     caCertKey,
			Hostnames: hostnames,
		}
		// TODO Not sure about the user...
		_, err := c.CertManager.EnsureCertificate(mkeAPIReq, constant.ApiserverUser)
		return err
	})

	return eg.Wait()
}

// Run does nothing, the cert component only needs to be initialized
func (c *Certificates) Run() error {
	return nil
}

// Stop does nothing, the cert component is not constantly running
func (c *Certificates) Stop() error {
	return nil
}

func kubeConfig(dest, url, caCert, clientCert, clientKey string) error {
	if util.FileExists(dest) {
		return nil
	}
	data := struct {
		URL        string
		CACert     string
		ClientCert string
		ClientKey  string
	}{
		URL:        url,
		CACert:     base64.StdEncoding.EncodeToString([]byte(caCert)),
		ClientCert: base64.StdEncoding.EncodeToString([]byte(clientCert)),
		ClientKey:  base64.StdEncoding.EncodeToString([]byte(clientKey)),
	}

	output, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer output.Close()

	return kubeconfigTemplate.Execute(output, &data)
}

func generateKeyPair(name string) error {
	keyFile := filepath.Join(constant.CertRootDir, fmt.Sprintf("%s.key", name))
	pubFile := filepath.Join(constant.CertRootDir, fmt.Sprintf("%s.pub", name))

	if util.FileExists(keyFile) && util.FileExists(pubFile) {
		return nil
	}

	reader := rand.Reader
	bitSize := 2048

	key, err := rsa.GenerateKey(reader, bitSize)
	if err != nil {
		return err
	}

	var privateKey = &pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(key),
	}

	outFile, err := os.Create(keyFile)
	if err != nil {
		return err
	}
	defer outFile.Close()

	err = pem.Encode(outFile, privateKey)
	if err != nil {
		return err
	}

	// note to the next reader: key.Public() != key.PublicKey
	pubBytes, err := x509.MarshalPKIXPublicKey(key.Public())
	if err != nil {
		return err
	}

	var pemkey = &pem.Block{
		Type:  "PUBLIC KEY",
		Bytes: pubBytes,
	}

	pemfile, err := os.Create(pubFile)
	if err != nil {
		return err
	}
	defer pemfile.Close()

	err = pem.Encode(pemfile, pemkey)
	if err != nil {
		return err
	}

	return nil
}

// Health-check interface
func (c *Certificates) Healthy() error { return nil }
