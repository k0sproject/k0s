package autopilot

import (
	"context"
	"fmt"

	"github.com/sirupsen/logrus"

	apcli "github.com/k0sproject/k0s/pkg/autopilot/client"
	apcont "github.com/k0sproject/k0s/pkg/autopilot/controller"
	aproot "github.com/k0sproject/k0s/pkg/autopilot/controller/root"
	"github.com/k0sproject/k0s/pkg/constant"
	"github.com/k0sproject/k0s/pkg/kubernetes"
)

func New(ctx context.Context, k0sVars constant.CfgVars, mode string, cf kubernetes.ClientFactoryInterface) (aproot.Root, error) {
	// TODO: rew it as a Component
	cfg := aproot.RootConfig{
		KubeConfig:          k0sVars.AdminKubeConfigPath,
		K0sDataDir:          k0sVars.DataDir,
		Mode:                mode,
		ManagerPort:         0,
		MetricsBindAddr:     "0",
		HealthProbeBindAddr: "0",
		ExcludeFromPlans:    nil,
	}

	cli, err := apcli.NewClientFactory(cf.GetRESTConfig())
	if err != nil {
		return nil, err
	}

	logger := logrus.NewEntry(logrus.New())
	switch cfg.Mode {
	case "controller":
		return apcont.NewRootController(cfg, logger, cli)
	case "worker":
		return apcont.NewRootWorker(cfg, logger, cli)
	}

	return nil, fmt.Errorf("unsupported root mode = '%s'", cfg.Mode)
}
