package server

import (
	"path"

	"github.com/Mirantis/mke/pkg/assets"
	"github.com/Mirantis/mke/pkg/constant"
	"github.com/Mirantis/mke/pkg/supervisor"
	"github.com/sirupsen/logrus"
	//	config "github.com/Mirantis/mke/pkg/apis/v1beta1"
)

// Etcd implement the component interface to run etcd
type Etcd struct {
	//	Config     *config.EtcdConfig
	supervisor supervisor.Supervisor
}

// Init extracts the needed binaries
func (k *Etcd) Init() error {
	return assets.Stage(constant.DataDir, path.Join("bin", "etcd"))
}

// Run runs etcd
func (k *Etcd) Run() error {
	logrus.Info("Starting etcd")

	k.supervisor = supervisor.Supervisor{
		Name:    "etcd",
		BinPath: path.Join(constant.DataDir, "bin", "etcd"),
		Dir:     constant.DataDir,
		Args: []string{
			"--data-dir=/var/lib/mke/etcd",
			"--listen-client-urls=https://127.0.0.1:2379",
			"--advertise-client-urls=https://127.0.0.1:2379",
			"--client-cert-auth=true",
			"--trusted-ca-file=/var/lib/mke/pki/etcd/ca.crt",
			"--cert-file=/var/lib/mke/pki/etcd/server.crt",
			"--key-file=/var/lib/mke/pki/etcd/server.key",
		},
	}

	k.supervisor.Supervise()

	return nil
}

// Stop stops etcd
func (k *Etcd) Stop() error {
	return k.supervisor.Stop()
}
