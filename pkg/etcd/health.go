/*
Copyright 2020 k0s authors

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

	return c.Health(ctx)
}
