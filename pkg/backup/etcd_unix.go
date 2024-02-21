//go:build unix

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
	"fmt"
	"os"
	"path/filepath"

	"go.etcd.io/etcd/client/v3/snapshot"
	"go.uber.org/zap"

	"github.com/k0sproject/k0s/internal/pkg/file"
	"github.com/k0sproject/k0s/pkg/etcd"

	utilsnapshot "go.etcd.io/etcd/etcdutl/v3/snapshot"
)

const etcdBackup = "etcd-snapshot.db"

type etcdStep struct {
	certRootDir string
	etcdCertDir string

	peerAddress string
	etcdDataDir string
	tmpDir      string
}

func newEtcdStep(tmpDir string, certRootDir string, etcdCertDir string, peerAddress string, etcdDataDir string) *etcdStep {
	return &etcdStep{tmpDir: tmpDir, certRootDir: certRootDir, etcdCertDir: etcdCertDir, peerAddress: peerAddress, etcdDataDir: etcdDataDir}
}

func (e etcdStep) Name() string {
	return "etcd"
}

func (e etcdStep) Backup() (StepResult, error) {
	ctx := context.TODO()
	etcdClient, err := etcd.NewClient(e.certRootDir, e.etcdCertDir, nil)
	if err != nil {
		return StepResult{}, err
	}
	path := filepath.Join(e.tmpDir, etcdBackup)

	// disable etcd's logging
	lg := zap.NewNop()

	// save snapshot
	if err = snapshot.Save(ctx, lg, *etcdClient.Config, path); err != nil {
		return StepResult{}, err
	}
	// add snapshot's path to assets
	return StepResult{filesForBackup: []string{path}}, nil
}

func (e etcdStep) Restore(restoreFrom, _ string) error {
	snapshotPath := filepath.Join(restoreFrom, etcdBackup)
	if !file.Exists(snapshotPath) {
		return fmt.Errorf("etcd snapshot not found at %s", snapshotPath)
	}

	// disable etcd's logging
	lg := zap.NewNop()
	m := utilsnapshot.NewV3(lg)
	name, err := os.Hostname()
	if err != nil {
		return err
	}
	peerURL := fmt.Sprintf("https://%s:2380", e.peerAddress)
	restoreConfig := utilsnapshot.RestoreConfig{
		SnapshotPath:   snapshotPath,
		OutputDataDir:  e.etcdDataDir,
		PeerURLs:       []string{peerURL},
		Name:           name,
		InitialCluster: fmt.Sprintf("%s=%s", name, peerURL),
	}

	err = m.Restore(restoreConfig)
	if err != nil {
		return err
	}

	return nil
}
