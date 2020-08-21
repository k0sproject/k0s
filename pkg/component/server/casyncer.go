package server

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"path/filepath"

	"github.com/Mirantis/mke/pkg/apis/v1beta1"
	"github.com/Mirantis/mke/pkg/constant"
	"github.com/Mirantis/mke/pkg/util"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"k8s.io/client-go/tools/clientcmd"
)

type CASyncer struct {
	JoinAddress string
	Token       string
	httpClient  http.Client
	bearerToken string
}

// Init initializes the CASyncer component
func (c *CASyncer) Init() error {
	data, err := base64.StdEncoding.DecodeString(c.Token)
	if err != nil {
		return errors.Wrapf(err, "failed to decode token")
	}
	clientConfig, err := clientcmd.NewClientConfigFromBytes(data)
	if err != nil {
		return err
	}
	config, err := clientConfig.ClientConfig()
	if err != nil {
		return err
	}

	ca := x509.NewCertPool()
	ca.AppendCertsFromPEM(config.CAData)
	tlsConfig := &tls.Config{
		InsecureSkipVerify: false,
		RootCAs:            ca,
	}
	tr := &http.Transport{TLSClientConfig: tlsConfig}
	c.httpClient = http.Client{Transport: tr}
	c.bearerToken = config.BearerToken
	logrus.Info("initialized CASyncer succesfully")
	return nil
}

// Run runs the CA sync process
func (c *CASyncer) Run() error {

	req, err := http.NewRequest(http.MethodGet, c.JoinAddress+"/v1beta1/ca", nil)
	if err != nil {
		return err
	}
	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", c.bearerToken))

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected response status: %s", resp.Status)
	}
	logrus.Info("got valid CA response")
	var caData v1beta1.CaResponse
	if resp.Body == nil {
		return fmt.Errorf("response body was nil !?!?")
	}
	defer resp.Body.Close()
	b, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	err = json.Unmarshal(b, &caData)
	if err != nil {
		return err
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
	keyFile := filepath.Join(constant.CertRoot, "ca.key")
	certFile := filepath.Join(constant.CertRoot, "ca.crt")

	if util.FileExists(keyFile) && util.FileExists(certFile) {
		logrus.Warnf("ca certs already exists, not gonna overwrite. If you wish to re-sync them, delete the existing ones.")
		return nil
	}

	err := ioutil.WriteFile(keyFile, caData.Key, 0600)
	if err != nil {
		return err
	}

	err = ioutil.WriteFile(certFile, caData.Cert, 0640)
	if err != nil {
		return err
	}

	return nil
}
