package v1beta1

import (
	"bytes"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"k8s.io/client-go/tools/clientcmd"
)

type JoinClient struct {
	joinAddress string
	httpClient  http.Client
	bearerToken string
}

// FromToken creates a new join api client from a token
func JoinClientFromToken(joinAddress, token string) (*JoinClient, error) {
	data, err := base64.StdEncoding.DecodeString(token)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to decode token")
	}
	clientConfig, err := clientcmd.NewClientConfigFromBytes(data)
	if err != nil {
		return nil, err
	}
	config, err := clientConfig.ClientConfig()
	if err != nil {
		return nil, err
	}

	ca := x509.NewCertPool()
	ca.AppendCertsFromPEM(config.CAData)
	tlsConfig := &tls.Config{
		InsecureSkipVerify: false,
		RootCAs:            ca,
	}
	tr := &http.Transport{TLSClientConfig: tlsConfig}
	c := &JoinClient{
		httpClient:  http.Client{Transport: tr},
		bearerToken: config.BearerToken,
	}
	c.joinAddress = joinAddress
	logrus.Info("initialized join client succesfully")
	return c, nil
}

func (j *JoinClient) GetCA() (CaResponse, error) {
	var caData CaResponse
	req, err := http.NewRequest(http.MethodGet, j.joinAddress+"/v1beta1/ca", nil)
	if err != nil {
		return caData, err
	}
	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", j.bearerToken))

	resp, err := j.httpClient.Do(req)
	if err != nil {
		return caData, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return caData, fmt.Errorf("unexpected response status: %s", resp.Status)
	}
	logrus.Info("got valid CA response")
	if resp.Body == nil {
		return caData, fmt.Errorf("response body was nil !?!?")
	}
	b, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return caData, err
	}
	err = json.Unmarshal(b, &caData)
	if err != nil {
		return caData, err
	}
	return caData, nil
}

func (j *JoinClient) JoinEtcd(peerAddress string) (EtcdResponse, error) {
	var etcdResponse EtcdResponse
	etcdRequest := EtcdRequest{
		PeerAddress: peerAddress,
	}
	name, err := os.Hostname()
	if err != nil {
		return etcdResponse, err
	}
	etcdRequest.Node = name

	buf := new(bytes.Buffer)
	json.NewEncoder(buf).Encode(etcdRequest)

	req, err := http.NewRequest(http.MethodPost, j.joinAddress+"/v1beta1/etcd", buf)
	if err != nil {
		return etcdResponse, err
	}
	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", j.bearerToken))
	resp, err := j.httpClient.Do(req)
	if err != nil {
		return etcdResponse, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return etcdResponse, fmt.Errorf("unexpected response status: %s", resp.Status)
	}

	b, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return etcdResponse, err
	}
	err = json.Unmarshal(b, &etcdResponse)
	if err != nil {
		return etcdResponse, err
	}

	return etcdResponse, nil
}
