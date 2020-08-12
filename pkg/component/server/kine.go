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
	k.uid, err = util.GetUid("kine")
	if err != nil {
		logrus.Warning(errors.Wrap(err, "Running kine as root"))
	}

	k.gid, _ = util.GetGid(constant.Group)

	dsURL, err := url.Parse(k.Config.DataSource)
	if err != nil {
		return err
	}
	if dsURL.Scheme == "sqlite" {
		// Make sure the db basedir exists
		err = os.MkdirAll(filepath.Dir(dsURL.Path), 0750)
		if err != nil {
			return errors.Wrapf(err, "failed to create dir %s", filepath.Dir(dsURL.Path))
		}
		err = os.Chown(filepath.Dir(dsURL.Path), k.uid, k.gid)
		if err != nil {
			return errors.Wrapf(err, "failed to chown dir %s", filepath.Dir(dsURL.Path))
		}
		os.Chown(dsURL.Path, k.uid, k.gid) // ignore error. file may not exist
	}
	return assets.Stage(constant.DataDir, path.Join("bin", "kine"), constant.Group)
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
		Uid: k.uid,
		Gid: k.gid,
	}

	k.supervisor.Supervise()

	return nil
}

// Stop stops kine
func (k *Kine) Stop() error {
	return k.supervisor.Stop()
}
