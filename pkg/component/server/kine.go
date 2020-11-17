/*
Copyright 2020 Mirantis, Inc.

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
package server

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"

	"github.com/k0sproject/k0s/pkg/assets"
	"github.com/k0sproject/k0s/pkg/constant"
	"github.com/k0sproject/k0s/pkg/supervisor"
	"github.com/k0sproject/k0s/pkg/util"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"

	config "github.com/k0sproject/k0s/pkg/apis/v1beta1"
)

var kineSocketDir = filepath.Dir(constant.KineSocketPath)

// Kine implement the component interface to run kine
type Kine struct {
	Config     *config.KineConfig
	supervisor supervisor.Supervisor
	uid        int
	gid        int
}

// Init extracts the needed binaries
func (k *Kine) Init() error {
	var err error
	k.uid, err = util.GetUID(constant.KineUser)
	if err != nil {
		logrus.Warning(errors.Wrap(err, "Running kine as root"))
	}

	err = util.InitDirectory(kineSocketDir, 0755)
	if err != nil {
		return errors.Wrapf(err, "failed to create %s", kineSocketDir)
	}
	if err := os.Chown(kineSocketDir, k.uid, k.gid); err != nil && os.Geteuid() == 0 {
		logrus.Warningf("failed to chown %s", kineSocketDir)
	}

	dsURL, err := url.Parse(k.Config.DataSource)
	if err != nil {
		return err
	}
	if dsURL.Scheme == "sqlite" {
		// Make sure the db basedir exists
		err = util.InitDirectory(filepath.Dir(dsURL.Path), 0750)
		if err != nil {
			return errors.Wrapf(err, "failed to create dir %s", filepath.Dir(dsURL.Path))
		}
		err = os.Chown(filepath.Dir(dsURL.Path), k.uid, k.gid)
		if err != nil && os.Geteuid() == 0 {
			return errors.Wrapf(err, "failed to chown dir %s", filepath.Dir(dsURL.Path))
		}
		if err := os.Chown(dsURL.Path, k.uid, k.gid); err != nil && os.Geteuid() == 0 {
			logrus.Warningf("datasource file %s does not exist", dsURL.Path)
		}
	}
	return assets.Stage(constant.BinDir, "kine", constant.BinDirMode)
}

// Run runs kine
func (k *Kine) Run() error {
	logrus.Info("Starting kine")
	logrus.Debugf("datasource: %s", k.Config.DataSource)

	k.supervisor = supervisor.Supervisor{
		Name:    "kine",
		BinPath: assets.BinPath("kine"),
		Dir:     constant.DataDir,
		Args: []string{
			fmt.Sprintf("--endpoint=%s", k.Config.DataSource),
			fmt.Sprintf("--listen-address=unix://%s", constant.KineSocketPath),
		},
		UID: k.uid,
		GID: k.gid,
	}

	k.supervisor.Supervise()

	return nil
}

// Stop stops kine
func (k *Kine) Stop() error {
	return k.supervisor.Stop()
}

// Health-check interface
func (k *Kine) Healthy() error { return nil }
