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

// Kine implement the component interface to run kine
type Kine struct {
	Config       *v1beta1.KineConfig
	gid          int
	K0sVars      *config.CfgVars
	supervisor   supervisor.Supervisor
	uid          int
	bypassClient *etcd.Client
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
	if err := os.Chown(kineSocketDir, k.uid, k.gid); err != nil && os.Geteuid() == 0 {
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
			err = os.Chown(dbDir, k.uid, k.gid)
			if err != nil && os.Geteuid() == 0 {
				return fmt.Errorf("failed to change ownership of SQLite database directory: %w", err)
			}
			if err := os.Chown(dbPath, k.uid, k.gid); err != nil && !errors.Is(err, os.ErrNotExist) && os.Geteuid() == 0 {
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
	return assets.Stage(k.K0sVars.BinDir, "kine", constant.BinDirMode)
}

// Run runs kine
func (k *Kine) Start(ctx context.Context) error {
	logrus.Info("Starting kine")

	k.supervisor = supervisor.Supervisor{
		Name:    "kine",
		BinPath: assets.BinPath("kine", k.K0sVars.BinDir),
		DataDir: k.K0sVars.DataDir,
		RunDir:  k.K0sVars.RunDir,
		Args: []string{
			fmt.Sprintf("--endpoint=%s", k.Config.DataSource),
			// NB: kine doesn't parse URLs properly, so construct potentially
			// invalid URLs that are understood by kine.
			// https://github.com/k3s-io/kine/blob/v0.13.2/pkg/util/network.go#L5-L13
			fmt.Sprintf("--listen-address=unix://%s", k.K0sVars.KineSocketPath),
			// Enable metrics on port 2380. The default is 8080, which clashes with kube-router.
			"--metrics-bind-address=:2380",
		},
		UID: k.uid,
		GID: k.gid,
	}

	return k.supervisor.Supervise()
}

// Stop stops kine
func (k *Kine) Stop() error {
	k.supervisor.Stop()
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
