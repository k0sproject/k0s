package component

import (
	"fmt"
	"path"

	"github.com/Mirantis/mke/pkg/assets"
	"github.com/Mirantis/mke/pkg/constant"
	"github.com/Mirantis/mke/pkg/supervisor"
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
	return assets.Stage(constant.DataDir, path.Join("bin","kine"))
}

// Run runs kine
func (k *Kine) Run() error {
	logrus.Info("Starting kine")
	logrus.Debugf("datasource: %s", k.Config.DataSource)
	k.supervisor = supervisor.Supervisor{
		Name:    "kine",
		BinPath: path.Join(constant.DataDir, "bin", "kine"),
		Dir:     constant.DataDir, // makes kine sqlite db to be created on /var/lib/mke/db
		Args: []string{
			// Does not work yet because of https://github.com/rancher/kine/blob/26bd5085f45f9c0c687d1ac652fa4526b07d2653/pkg/endpoint/endpoint.go#L145
			// kine will only always default to /.db ... :()
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
