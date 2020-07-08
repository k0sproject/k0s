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
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"

	config "github.com/Mirantis/mke/pkg/apis/v1beta1"
)

// Kine implement the component interface to run kine
type Kine struct {
	Config     *config.KineConfig
	supervisor supervisor.Supervisor
}

// Init extracts the needed binaries
func (k *Kine) Init() error {
	dsURL, err := url.Parse(k.Config.DataSource)
	if err != nil {
		return err
	}
	if dsURL.Scheme == "sqlite" {
		// Make sure the db basedir exists
		err = os.MkdirAll(filepath.Dir(dsURL.Path), 0700)
		if err != nil {
			return errors.Wrapf(err, "failed to create dir %s", filepath.Dir(dsURL.Path))
		}
	}
	return assets.Stage(constant.DataDir, path.Join("bin", "kine"))
}

// Run runs kine
func (k *Kine) Run() error {
	logrus.Info("Starting kine")
	logrus.Debugf("datasource: %s", k.Config.DataSource)

	k.supervisor = supervisor.Supervisor{
		Name:    "kine",
		BinPath: path.Join(constant.DataDir, "bin", "kine"),
		Dir:     constant.DataDir,
		Args: []string{
			fmt.Sprintf("--endpoint=%s", k.Config.DataSource),
		},
	}

	k.supervisor.Supervise()

	return nil
}

// Stop stops kine
func (k *Kine) Stop() error {
	return k.supervisor.Stop()
}
