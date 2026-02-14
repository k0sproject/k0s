// SPDX-FileCopyrightText: 2020 k0s authors
// SPDX-License-Identifier: Apache-2.0

package etcd

import (
	"context"

	"github.com/k0sproject/k0s/pkg/apis/k0s/v1beta1"

	"github.com/sirupsen/logrus"
)

// CheckEtcdReady returns true if etcd responds to the metrics endpoint with a status code of 200
func CheckEtcdReady(ctx context.Context, certDir string, etcdCertDir string, etcdConf *v1beta1.EtcdConfig) error {
	c, err := NewClient(certDir, etcdCertDir, etcdConf)
	if err != nil {
		logrus.Errorf("failed to initialize etcd client: %v", err)
		return err
	}
	defer c.Close()

	return c.Health(ctx)
}
