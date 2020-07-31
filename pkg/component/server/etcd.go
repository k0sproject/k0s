package server

import (
	"fmt"
	"os"
	"path"

	"github.com/Mirantis/mke/pkg/assets"
	"github.com/Mirantis/mke/pkg/constant"
	"github.com/Mirantis/mke/pkg/supervisor"
	"github.com/Mirantis/mke/pkg/util"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	//	config "github.com/Mirantis/mke/pkg/apis/v1beta1"
)

// Etcd implement the component interface to run etcd
type Etcd struct {
	//	Config     *config.EtcdConfig
	supervisor  supervisor.Supervisor
	uid         int
	gid         int
	etcdDataDir string
	certDir     string
}

// Init extracts the needed binaries
func (e *Etcd) Init() error {
	var err error
	e.uid, err = util.GetUid(constant.EtcdUser)
	if err != nil {
		logrus.Warning(errors.Wrap(err, "Running etcd as root"))
	}

	e.etcdDataDir = path.Join(constant.DataDir, "etcd")
	err = os.MkdirAll(e.etcdDataDir, 0700)
	if err != nil {
		return errors.Wrapf(err, "failed to create %s", e.etcdDataDir)
	}

	e.gid, _ = util.GetGid(constant.Group)

	err = os.Chown(e.etcdDataDir, e.uid, e.gid)
	if err != nil {
		return errors.Wrapf(err, "failed to chown %s", e.etcdDataDir)
	}

	e.certDir = path.Join(constant.CertRoot, "etcd")
	os.Chown(path.Join(e.certDir, "ca.crt"), e.uid, e.gid)
	os.Chown(path.Join(e.certDir, "server.crt"), e.uid, e.gid)
	os.Chown(path.Join(e.certDir, "server.key"), e.uid, e.gid)

	return assets.Stage(constant.DataDir, path.Join("bin", "etcd"), constant.Group)
}

// Run runs etcd
func (e *Etcd) Run() error {
	logrus.Info("Starting etcd")

	e.supervisor = supervisor.Supervisor{
		Name:    "etcd",
		BinPath: path.Join(constant.DataDir, "bin", "etcd"),
		Dir:     constant.DataDir,
		Args: []string{
			fmt.Sprintf("--data-dir=%s", e.etcdDataDir),
			"--listen-client-urls=https://127.0.0.1:2379",
			"--advertise-client-urls=https://127.0.0.1:2379",
			"--client-cert-auth=true",
			fmt.Sprintf("--trusted-ca-file=%s", path.Join(e.certDir, "ca.crt")),
			fmt.Sprintf("--cert-file=%s", path.Join(e.certDir, "server.crt")),
			fmt.Sprintf("--key-file=%s", path.Join(e.certDir, "server.key")),
		},
		Uid: e.uid,
		Gid: e.gid,
	}

	e.supervisor.Supervise()

	return nil
}

// Stop stops etcd
func (e *Etcd) Stop() error {
	return e.supervisor.Stop()
}
