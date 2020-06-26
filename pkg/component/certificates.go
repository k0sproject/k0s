package component

import (
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

	"github.com/Mirantis/mke/pkg/constant"
	"github.com/Mirantis/mke/pkg/util"
	"github.com/cloudflare/cfssl/cli"
	"github.com/cloudflare/cfssl/cli/genkey"
	"github.com/cloudflare/cfssl/cli/sign"
	"github.com/cloudflare/cfssl/csr"
	"github.com/cloudflare/cfssl/initca"
	"github.com/cloudflare/cfssl/signer"
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

type Cert struct {
	Key  string
	Cert string
}

type Certificates struct {
	CACert string
	Certs  map[string]Cert
}

func (c *Certificates) Run() error {

	c.Certs = make(map[string]Cert)

	// Common CA
	if err := c.loadOrGenerateCA("ca", "kubernetes-ca"); err != nil {
		return err
	}

	caCertPath, caCertKey := filepath.Join(constant.CertRoot, "ca.crt"), filepath.Join(constant.CertRoot, "ca.key")
	// We need CA cert loaded to generate client configs
	logrus.Debugf("CA key and cert exists, loading")
	cert, err := ioutil.ReadFile(caCertPath)
	if err != nil {
		return errors.Wrapf(err, "failed to read ca cert")
	}
	c.CACert = string(cert)

	// Front proxy CA
	if err := c.loadOrGenerateCA("front-proxy-ca", "kubernetes-front-proxy-ca"); err != nil {
		return err
	}

	proxyCertPath, proxyCertKey := filepath.Join(constant.CertRoot, "front-proxy-ca.crt"), filepath.Join(constant.CertRoot, "front-proxy-ca.key")

	proxyClientReq := certReq{
		name:   "front-proxy-client",
		cn:     "front-proxy-client",
		o:      "front-proxy-client",
		caCert: proxyCertPath,
		caKey:  proxyCertKey,
	}
	if err := c.loadOrGenerateCert(proxyClientReq); err != nil {
		return err
	}

	// admin cert & kubeconfig
	adminReq := certReq{
		name:   "admin",
		cn:     "kubernetes-admin",
		o:      "system:masters",
		caCert: caCertPath,
		caKey:  caCertKey,
	}
	if err := c.loadOrGenerateCert(adminReq); err != nil {
		return err
	}
	if err := kubeConfig(filepath.Join(constant.CertRoot, "admin.conf"), "https://localhost:6443", c.CACert, c.Certs["admin"].Cert, c.Certs["admin"].Key); err != nil {
		return err
	}

	if err := generateKeyPair("sa"); err != nil {
		return err
	}

	ccmReq := certReq{
		name:   "ccm",
		cn:     "system:kube-controller-manager",
		o:      "system:kube-controller-manager",
		caCert: caCertPath,
		caKey:  caCertKey,
	}
	if err := c.loadOrGenerateCert(ccmReq); err != nil {
		return err
	}

	if err := kubeConfig(filepath.Join(constant.CertRoot, "ccm.conf"), "https://localhost:6443", c.CACert, c.Certs["ccm"].Cert, c.Certs["ccm"].Key); err != nil {
		return err
	}

	schedulerReq := certReq{
		name:   "scheduler",
		cn:     "system:kube-scheduler",
		o:      "system:kube-scheduler",
		caCert: caCertPath,
		caKey:  caCertKey,
	}
	if err := c.loadOrGenerateCert(schedulerReq); err != nil {
		return err
	}

	if err := kubeConfig(filepath.Join(constant.CertRoot, "scheduler.conf"), "https://localhost:6443", c.CACert, c.Certs["scheduler"].Cert, c.Certs["scheduler"].Key); err != nil {
		return err
	}

	kubeletClientReq := certReq{
		name:   "apiserver-kubelet-client",
		cn:     "apiserver-kubelet-client",
		o:      "system:masters",
		caCert: caCertPath,
		caKey:  caCertKey,
	}
	if err := c.loadOrGenerateCert(kubeletClientReq); err != nil {
		return err
	}

	hostnames := []string{
		"kubernetes",
		"kubernetes.default",
		"kubernetes.default.svc",
		"kubernetes.default.svc.cluster",
		"kubernetes.svc.cluster.local",
		"127.0.0.1",
		"localhost",
		// TODO add way to configure DNS names and public IPs etc.
	}

	serverReq := certReq{
		name:      "server",
		cn:        "kubernetes",
		o:         "kubernetes",
		caCert:    caCertPath,
		caKey:     caCertKey,
		hostnames: hostnames,
	}
	if err := c.loadOrGenerateCert(serverReq); err != nil {
		return err
	}

	// Generate the needed kubeconfigs too

	return nil
}

func (c *Certificates) Stop() error {
	return nil
}

func (c *Certificates) loadOrGenerateCA(name, commonName string) error {
	keyFile := filepath.Join(constant.CertRoot, fmt.Sprintf("%s.key", name))
	certFile := filepath.Join(constant.CertRoot, fmt.Sprintf("%s.crt", name))

	if util.FileExists(keyFile) && util.FileExists(certFile) {

		return nil
	}

	err := os.MkdirAll(constant.CertRoot, 0700)
	if err != nil {
		return errors.Wrapf(err, "failed to create pki dir")
	}

	req := new(csr.CertificateRequest)
	req.KeyRequest = csr.NewKeyRequest()
	req.KeyRequest.A = "rsa"
	req.KeyRequest.S = 2048
	req.CN = commonName
	req.CA = &csr.CAConfig{
		Expiry: "87600h",
	}
	cert, _, key, err := initca.New(req)
	if err != nil {
		return err
	}

	err = ioutil.WriteFile(keyFile, key, 0600)
	err = ioutil.WriteFile(certFile, cert, 0600)

	return err
}

type certReq struct {
	name      string
	cn        string
	o         string
	caKey     string
	caCert    string
	hostnames []string
}

func (c *Certificates) loadOrGenerateCert(certReq certReq) error {
	keyFile := filepath.Join(constant.CertRoot, fmt.Sprintf("%s.key", certReq.name))
	certFile := filepath.Join(constant.CertRoot, fmt.Sprintf("%s.crt", certReq.name))

	if util.FileExists(keyFile) && util.FileExists(certFile) {
		cert, err := ioutil.ReadFile(certFile)
		key, err := ioutil.ReadFile(keyFile)
		if err != nil {
			return errors.Wrapf(err, "failed to read ca cert or key for %s", certReq.name)
		}

		c.Certs[certReq.name] = Cert{
			Key:  string(key),
			Cert: string(cert),
		}
		return nil
	}

	req := csr.CertificateRequest{
		KeyRequest: csr.NewKeyRequest(),
		CN:         certReq.cn,
		Names: []csr.Name{
			csr.Name{O: certReq.o},
		},
	}

	req.KeyRequest.A = "rsa"
	req.KeyRequest.S = 2048
	req.Hosts = certReq.hostnames

	var key, csrBytes []byte
	g := &csr.Generator{Validator: genkey.Validator}
	csrBytes, key, err := g.ProcessRequest(&req)
	if err != nil {
		key = nil
		return err
	}
	config := cli.Config{
		CAFile:    certReq.caCert, //filepath.Join(constant.CertRoot, "ca.crt"),
		CAKeyFile: certReq.caKey,  //filepath.Join(constant.CertRoot, "ca.key"),
	}
	s, err := sign.SignerFromConfig(config)
	if err != nil {
		return err
	}

	var cert []byte
	signReq := signer.SignRequest{
		Request: string(csrBytes),
		Profile: "kubernetes",
	}

	cert, err = s.Sign(signReq)
	if err != nil {
		return err
	}
	if err != nil {
		return err
	}
	c.Certs[certReq.name] = Cert{
		Key:  string(key),
		Cert: string(cert),
	}
	err = ioutil.WriteFile(keyFile, key, 0600)
	err = ioutil.WriteFile(certFile, cert, 0600)

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
	keyFile := filepath.Join(constant.CertRoot, fmt.Sprintf("%s.key", name))
	pubFile := filepath.Join(constant.CertRoot, fmt.Sprintf("%s.pub", name))

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
