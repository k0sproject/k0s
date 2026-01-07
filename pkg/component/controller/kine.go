// SPDX-FileCopyrightText: 2020 k0s authors
// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"time"

	"github.com/k0sproject/k0s/pkg/apis/k0s/v1beta1"
	"github.com/k0sproject/k0s/pkg/component/manager"
	"github.com/k0sproject/k0s/pkg/config"
	"github.com/k0sproject/k0s/pkg/config/kine"
	"github.com/k0sproject/k0s/pkg/etcd"
	clientv3 "go.etcd.io/etcd/client/v3"

	"github.com/sirupsen/logrus"

	"github.com/k0sproject/k0s/internal/pkg/dir"
	"github.com/k0sproject/k0s/internal/pkg/users"
	"github.com/k0sproject/k0s/pkg/assets"
	"github.com/k0sproject/k0s/pkg/constant"
	"github.com/k0sproject/k0s/pkg/supervisor"
)

const kineGID = 0

// Kine implement the component interface to run kine
type Kine struct {
	Config  *v1beta1.KineConfig
	K0sVars *config.CfgVars

	supervisor     *supervisor.Supervisor
	executablePath string
	uid            int
	bypassClient   *etcd.Client
}

var _ manager.Component = (*Kine)(nil)
var _ manager.Ready = (*Kine)(nil)

// Init extracts the needed binaries
func (k *Kine) Init(_ context.Context) error {
	logrus.Infof("initializing kine")
	var err error
	k.uid, err = users.LookupUID(constant.KineUser)
	if err != nil {
		err = fmt.Errorf("failed to lookup UID for %q: %w", constant.KineUser, err)
		k.uid = users.RootUID
		logrus.WithError(err).Warn("Running kine as root")
	}

	kineSocketDir := filepath.Dir(k.K0sVars.KineSocketPath)
	err = dir.Init(kineSocketDir, 0755)
	if err != nil {
		return fmt.Errorf("failed to create %s: %w", kineSocketDir, err)
	}
	if err := os.Chown(kineSocketDir, k.uid, kineGID); err != nil && os.Geteuid() == 0 {
		logrus.Warn("failed to chown ", kineSocketDir)
	}

	if backend, dsn, err := kine.SplitDataSource(k.Config.DataSource); err != nil {
		return fmt.Errorf("unsupported kine data source: %w", err)
	} else if backend == "sqlite" {
		dbPath, err := kine.GetSQLiteFilePath(k.K0sVars.DataDir, dsn)
		if err != nil {
			logrus.WithError(err).Debug("Skipping SQLite database file initialization")
		} else {
			// Make sure the db basedir exists
			dbDir := filepath.Dir(dbPath)
			err = dir.Init(dbDir, constant.KineDBDirMode)
			if err != nil {
				return fmt.Errorf("failed to initialize SQLite database directory: %w", err)
			}
			err = os.Chown(dbDir, k.uid, kineGID)
			if err != nil && os.Geteuid() == 0 {
				return fmt.Errorf("failed to change ownership of SQLite database directory: %w", err)
			}
			if err := os.Chown(dbPath, k.uid, kineGID); err != nil && !errors.Is(err, os.ErrNotExist) && os.Geteuid() == 0 {
				logrus.WithError(err).Warn("Failed to change ownership of SQLite database file")
			}
		}
	}

	k.bypassClient, err = etcd.NewClientWithConfig(clientv3.Config{
		Endpoints: []string{(&url.URL{
			Scheme: "unix", OmitHost: true,
			Path: filepath.ToSlash(k.K0sVars.KineSocketPath),
		}).String()},
	})
	if err != nil {
		return fmt.Errorf("can't create bypass etcd client: %w", err)
	}
	k.executablePath, err = assets.StageExecutable(k.K0sVars.BinDir, "kine")
	return err
}

// Run runs kine
func (k *Kine) Start(ctx context.Context) error {
	logrus.Info("Starting kine")

	k.supervisor = &supervisor.Supervisor{
		Name:    "kine",
		BinPath: k.executablePath,
		DataDir: k.K0sVars.DataDir,
		RunDir:  k.K0sVars.RunDir,
		Args: []string{
			"--endpoint=" + k.Config.DataSource,
			// NB: kine doesn't parse URLs properly, so construct potentially
			// invalid URLs that are understood by kine.
			// https://github.com/k3s-io/kine/blob/v0.14.10/pkg/util/network.go#L5-L13
			"--listen-address=unix://" + k.K0sVars.KineSocketPath,
			// Enable metrics on port 2380. The default is 8080, which clashes with kube-router.
			"--metrics-bind-address=:2380",
			// https://github.com/k3s-io/kine/pull/513
			"--compact-interval=0",
		},
		UID: k.uid,
		GID: kineGID,
	}

	return k.supervisor.Supervise(ctx)
}

// Stop stops kine
func (k *Kine) Stop() error {
	if k.supervisor != nil {
		return k.supervisor.Stop()
	}
	return nil
}

const hcKey = "/k0s-health-check"
const hcValue = "value"

func (k *Kine) Ready() error {
	ctx, cancel := context.WithTimeout(context.TODO(), 1*time.Second)
	defer cancel()

	ok, err := k.bypassClient.Write(ctx, hcKey, hcValue, 64*time.Second)
	if err != nil {
		return fmt.Errorf("kine-etcd-health: %w", err)
	}
	if !ok {
		logrus.Warningf("kine-etcd-health: health-check value was not written")
	}

	v, err := k.bypassClient.Read(ctx, hcKey)
	if err != nil {
		return fmt.Errorf("kine-etcd-health read: %w", err)
	}
	if realValue := string(v.Kvs[len(v.Kvs)-1].Value); realValue != hcValue {
		return fmt.Errorf("kine-etcd-health read: value is invalid, got %s, expect %s", realValue, hcValue)
	}
	return nil
}
