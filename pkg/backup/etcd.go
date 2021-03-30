/*
Copyright 2021 k0s authors

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
package backup

import (
	"context"
	"path/filepath"

	"go.etcd.io/etcd/clientv3/snapshot"
	"go.uber.org/zap"

	"github.com/k0sproject/k0s/pkg/etcd"
)

func (c *Config) saveEtcdSnapshot() error {
	ctx := context.TODO()
	etcdClient, err := etcd.NewClient(c.k0sVars.CertRootDir, c.k0sVars.EtcdCertDir)
	if err != nil {
		return err
	}
	path := filepath.Join(c.tmpDir, "etcd-snapshot.db")

	// disable etcd's logging
	lg := zap.NewNop()
	m := snapshot.NewV3(lg)

	// save snapshot
	if err = m.Save(ctx, *etcdClient.Config, path); err != nil {
		return err
	}
	// add snapshot's path to assets
	c.savedAssets = append(c.savedAssets, path)
	return nil
}
