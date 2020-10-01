package certificate

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

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
	keyFile := filepath.Join(constant.CertRoot, fmt.Sprintf("%s.key", name))
	certFile := filepath.Join(constant.CertRoot, fmt.Sprintf("%s.crt", name))

	if util.FileExists(keyFile) && util.FileExists(certFile) {

		return nil
	}

	err := util.InitDirectory(filepath.Dir(keyFile), 0750)
	if err != nil {
		return errors.Wrapf(err, "failed to create pki dir")
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

	err = ioutil.WriteFile(keyFile, key, 0600)
	if err != nil {
		return err
	}

	err = ioutil.WriteFile(certFile, cert, 0640)
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

	keyFile := filepath.Join(constant.CertRoot, fmt.Sprintf("%s.key", certReq.Name))
	certFile := filepath.Join(constant.CertRoot, fmt.Sprintf("%s.crt", certReq.Name))

	gid, err := util.GetGID(constant.Group)
	uid, err := util.GetUID(ownerName)

	if util.FileExists(keyFile) && util.FileExists(certFile) {
		_ = os.Chown(keyFile, uid, gid)
		_ = os.Chown(certFile, uid, gid)

		cert, err := ioutil.ReadFile(certFile)
		key, err := ioutil.ReadFile(keyFile)
		if err != nil {
			return Certificate{}, errors.Wrapf(err, "failed to read ca cert or key for %s", certReq.Name)
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
			csr.Name{O: certReq.O},
		},
	}

	req.KeyRequest.A = "rsa"
	req.KeyRequest.S = 2048
	req.Hosts = certReq.Hostnames

	var key, csrBytes []byte
	g := &csr.Generator{Validator: genkey.Validator}
	csrBytes, key, err = g.ProcessRequest(&req)
	if err != nil {
		key = nil
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
	if err != nil {
		return Certificate{}, err
	}
	c := Certificate{
		Key:  string(key),
		Cert: string(cert),
	}
	err = ioutil.WriteFile(keyFile, key, 0600)
	err = ioutil.WriteFile(certFile, cert, 0640)

	err = os.Chown(keyFile, uid, gid)
	err = os.Chown(certFile, uid, gid)

	return c, err
}
