package etcd

import (
	"context"

	"github.com/sirupsen/logrus"
)

// CheckEtcdReady returns true if etcd responds to the metrics endpoint with a status code of 200
func CheckEtcdReady(ctx context.Context, certDir string, etcdCertDir string) error {
	c, err := NewClient(certDir, etcdCertDir)
	if err != nil {
		logrus.Errorf("failed to initialize etcd client: %v", err)
		return err
	}

	return c.Health(ctx)
}
