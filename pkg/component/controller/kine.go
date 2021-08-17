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
package controller

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"

	"github.com/k0sproject/k0s/pkg/apis/k0s.k0sproject.io/v1beta1"

	"github.com/sirupsen/logrus"

	"github.com/k0sproject/k0s/internal/pkg/dir"
	"github.com/k0sproject/k0s/internal/pkg/users"
	"github.com/k0sproject/k0s/pkg/assets"
	"github.com/k0sproject/k0s/pkg/constant"
	"github.com/k0sproject/k0s/pkg/supervisor"
)

// Kine implement the component interface to run kine
type Kine struct {
	Config     *v1beta1.KineConfig
	gid        int
	K0sVars    constant.CfgVars
	supervisor supervisor.Supervisor
	uid        int
}

// Init extracts the needed binaries
func (k *Kine) Init() error {
	var err error
	k.uid, err = users.GetUID(constant.KineUser)
	if err != nil {
		logrus.Warning(fmt.Errorf("running kine as root: %w", err))
	}

	kineSocketDir := filepath.Dir(k.K0sVars.KineSocketPath)
	err = dir.Init(kineSocketDir, 0755)
	if err != nil {
		return fmt.Errorf("failed to create %s: %w", kineSocketDir, err)
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
		err = dir.Init(filepath.Dir(dsURL.Path), constant.KineDBDirMode)
		if err != nil {
			return fmt.Errorf("failed to create dir %s: %w", filepath.Dir(dsURL.Path), err)
		}
		err = os.Chown(filepath.Dir(dsURL.Path), k.uid, k.gid)
		if err != nil && os.Geteuid() == 0 {
			return fmt.Errorf("failed to chown dir %s: %w", filepath.Dir(dsURL.Path), err)
		}
		if err := os.Chown(dsURL.Path, k.uid, k.gid); err != nil && os.Geteuid() == 0 {
			logrus.Warningf("datasource file %s does not exist", dsURL.Path)
		}
	}
	return assets.Stage(k.K0sVars.BinDir, "kine", constant.BinDirMode)
}

// Run runs kine
func (k *Kine) Run() error {
	logrus.Info("Starting kine")
	logrus.Debugf("datasource: %s", k.Config.DataSource)

	k.supervisor = supervisor.Supervisor{
		Name:    "kine",
		BinPath: assets.BinPath("kine", k.K0sVars.BinDir),
		DataDir: k.K0sVars.DataDir,
		RunDir:  k.K0sVars.RunDir,
		Args: []string{
			fmt.Sprintf("--endpoint=%s", k.Config.DataSource),
			fmt.Sprintf("--listen-address=unix://%s", k.K0sVars.KineSocketPath),
		},
		UID: k.uid,
		GID: k.gid,
	}

	return k.supervisor.Supervise()
}

// Stop stops kine
func (k *Kine) Stop() error {
	return k.supervisor.Stop()
}

// Reconcile detects changes in configuration and applies them to the component
func (k *Kine) Reconcile() error {
	logrus.Debug("reconcile method called for: Kine")
	return nil
}

// Health-check interface
func (k *Kine) Healthy() error { return nil }
