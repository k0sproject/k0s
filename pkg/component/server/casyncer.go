package server

import (
	"io/ioutil"
	"path/filepath"

	"github.com/Mirantis/mke/pkg/apis/v1beta1"
	"github.com/Mirantis/mke/pkg/constant"
	"github.com/Mirantis/mke/pkg/util"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

// CASyncer is the Component implementation to sync CAs between multiple controllers
type CASyncer struct {
	JoinClient *v1beta1.JoinClient
}

// Init initializes the CASyncer component
func (c *CASyncer) Init() error {

	return nil
}

// Run runs the CA sync process
func (c *CASyncer) Run() error {
	caData, err := c.JoinClient.GetCA()
	if err != nil {
		return errors.Wrapf(err, "failed to sync CA")
	}
	// Dump certs into files
	return writeCerts(caData)
}

// Stop does nothing, there's nothing running constantly
func (c *CASyncer) Stop() error {
	// Nothing to do
	return nil
}

func writeCerts(caData v1beta1.CaResponse) error {
	keyFile := filepath.Join(constant.CertRootDir, "ca.key")
	certFile := filepath.Join(constant.CertRootDir, "ca.crt")

	if util.FileExists(keyFile) && util.FileExists(certFile) {
		logrus.Warnf("ca certs already exists, not gonna overwrite. If you wish to re-sync them, delete the existing ones.")
		return nil
	}

	err := ioutil.WriteFile(keyFile, caData.Key, constant.CertSecureMode)
	if err != nil {
		return err
	}

	err = ioutil.WriteFile(certFile, caData.Cert, constant.CertMode)
	if err != nil {
		return err
	}

	return nil
}

// Health-check interface
func (c *CASyncer) Healthy() error { return nil }
