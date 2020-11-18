package etcd

import (
	"context"
	"net/http"
	"net/url"
	"path/filepath"
	"time"

	"github.com/sirupsen/logrus"
	"go.etcd.io/etcd/pkg/transport"
)

// CheckEtcdReady returns true if etcd responds to the metrics endpoint with a status code of 200
func CheckEtcdReady(certDir string, etcdCertDir string) error {
	c, err := NewClient(certDir, etcdCertDir)
	if err != nil {
		logrus.Errorf("failed to initialize etcd client: %v", err)
		return err
	}
	memberList, err := c.client.MemberList(context.Background())
	if err != nil {
		logrus.Errorf("failed to fetch etcd member list: %v\n", err)
		return err
	}

	u, err := url.Parse(memberList.Members[0].ClientURLs[0])
	if err != nil {
		logrus.Errorf("cannot fetch health endpoint: %v\n", err)
		return err
	}

	// the metrics endpoint was selected as a health endpoint in the official etcd docs: https://etcd.io/docs/v3.4.0/op-guide/monitoring/
	u.Path = "/metrics"

	tr, err := transport.NewTransport(transport.TLSInfo{
		CertFile:      filepath.Join(certDir, "apiserver-etcd-client.crt"),
		KeyFile:       filepath.Join(certDir, "apiserver-etcd-client.key"),
		TrustedCAFile: filepath.Join(etcdCertDir, "ca.crt"),
	}, 5*time.Second)
	if err != nil {
		logrus.Errorf("error encountered setting up healthcheck TLS config: %v\n", err)
	}

	resp, err := tr.RoundTrip(&http.Request{
		Header: make(http.Header),
		Method: http.MethodGet,
		URL:    u,
	})
	if err != nil {
		logrus.Errorf("error accessing health endpoint: %v\n", err)
		return err
	}
	if resp.StatusCode != http.StatusOK {
		logrus.Printf("received unexpected status code from endpoint. expected %v, received %v", http.StatusOK, resp.StatusCode)
	}

	return nil
}
