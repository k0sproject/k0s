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
	"path"
	"path/filepath"

	"github.com/Mirantis/mke/pkg/assets"
	"github.com/Mirantis/mke/pkg/constant"
	"github.com/Mirantis/mke/pkg/supervisor"
	"github.com/Mirantis/mke/pkg/util"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"

	config "github.com/Mirantis/mke/pkg/apis/v1beta1"
)

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

	k.gid, _ = util.GetGID(constant.Group)

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
		if err != nil {
			return errors.Wrapf(err, "failed to chown dir %s", filepath.Dir(dsURL.Path))
		}
		if err := os.Chown(dsURL.Path, k.uid, k.gid); err != nil {
			logrus.Warningf("datasource file %s does not exist", dsURL.Path)
		}
	}
	return assets.Stage(constant.BinDir, "kine", constant.BinDirMode, constant.Group)
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
			fmt.Sprintf("--listen-address=unix://%s", path.Join(constant.RunDir, "kine.sock:2379")),
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
