package worker

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"github.com/davecgh/go-spew/spew"
	"github.com/k0sproject/k0s/pkg/token"
	"github.com/sirupsen/logrus"
	"io/ioutil"
	"k8s.io/client-go/tools/clientcmd"
	"net/http"
	"os"
)

type CalicoInstaller struct {
	token      string
	apiAddress string
}

func NewCalicoInstaller(t string, address string) *CalicoInstaller {
	return &CalicoInstaller{
		token:      t,
		apiAddress: address,
	}
}

func (c CalicoInstaller) Init() error {
	path := "C:\\bootstrap.ps1"

	if err := os.Mkdir("C:\\CalicoWindows", 777); err != nil {
		if os.IsExist(err) {
			logrus.Warn("CalicoWindows already set up")
			return nil
		}
		return fmt.Errorf("can't create CalicoWindows dir: %v", err)
	}

	if err := ioutil.WriteFile(path, []byte(installCalicoPowershell), 777); err != nil {
		return fmt.Errorf("can't unpack calico installer: %v", err)
	}

	if err := c.SaveKubeConfig("C:\\calico-kube-config"); err != nil {
		return fmt.Errorf("can't get calico-kube-config: %v", err)
	}

	return nil
}

func (c CalicoInstaller) SaveKubeConfig(path string) error {
	tokenBytes, err := token.JoinDecode(c.token)
	if err != nil {
		return fmt.Errorf("failed to decode token: %v", err)
	}
	spew.Dump(string(tokenBytes))
	clientConfig, err := clientcmd.NewClientConfigFromBytes(tokenBytes)
	if err != nil {
		return fmt.Errorf("failed to create api client config: %v", err)
	}
	config, err := clientConfig.ClientConfig()
	if err != nil {
		return fmt.Errorf("failed to create api client config: %v", err)
	}

	ca := x509.NewCertPool()
	ca.AppendCertsFromPEM(config.CAData)
	tlsConfig := &tls.Config{
		InsecureSkipVerify: false,
		RootCAs:            ca,
	}
	tr := &http.Transport{TLSClientConfig: tlsConfig}
	client := http.Client{Transport: tr}
	req, err := http.NewRequest(http.MethodGet, c.apiAddress+"/v1beta1/calico/kubeconfig", nil)
	if err != nil {
		return fmt.Errorf("can't create http request: %v", err)
	}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("can't download kubelet config for calico: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected response status: %s", resp.Status)
	}
	b, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("can't read response body: %v", err)
	}
	if err := ioutil.WriteFile(path, b, 0700); err != nil {
		return fmt.Errorf("can't save kubeconfig for calico: %v", err)
	}
	posh := New()
	return posh.execute("C:\\bootstrap.ps1 -KubeVersion 1.19.3")
}

func (c CalicoInstaller) Run() error {

	return nil
}

func (c CalicoInstaller) Stop() error {
	panic("implement me")
}

func (c CalicoInstaller) Healthy() error {
	panic("implement me")
}
