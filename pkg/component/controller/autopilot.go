/*
Copyright 2022 k0s authors

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
	"fmt"

	apcli "github.com/k0sproject/k0s/pkg/autopilot/client"
	apcont "github.com/k0sproject/k0s/pkg/autopilot/controller"
	aproot "github.com/k0sproject/k0s/pkg/autopilot/controller/root"
	"github.com/k0sproject/k0s/pkg/component/manager"
	"github.com/k0sproject/k0s/pkg/config"

	"github.com/k0sproject/k0s/pkg/kubernetes"
	"github.com/sirupsen/logrus"
)

var _ manager.Component = (*Autopilot)(nil)

type Autopilot struct {
	K0sVars            *config.CfgVars
	AdminClientFactory kubernetes.ClientFactoryInterface
	EnableWorker       bool
}

func (a *Autopilot) Init(ctx context.Context) error {
	return nil
}

func (a *Autopilot) Start(ctx context.Context) error {
	log := logrus.WithFields(logrus.Fields{"component": "autopilot"})

	autopilotClientFactory, err := apcli.NewClientFactory(a.AdminClientFactory.GetRESTConfig())
	if err != nil {
		return fmt.Errorf("creating autopilot client factory error: %w", err)
	}

	autopilotRoot, err := apcont.NewRootController(aproot.RootConfig{
		KubeConfig:          a.K0sVars.AdminKubeConfigPath,
		K0sDataDir:          a.K0sVars.DataDir,
		Mode:                "controller",
		ManagerPort:         8899,
		MetricsBindAddr:     "0",
		HealthProbeBindAddr: "0",
	}, logrus.WithFields(logrus.Fields{"component": "autopilot"}), a.EnableWorker, a.AdminClientFactory, autopilotClientFactory)
	if err != nil {
		return fmt.Errorf("failed to create autopilot controller: %w", err)
	}

	go func() {
		if err := autopilotRoot.Run(ctx); err != nil {
			log.WithError(err).Error("Error while running autopilot")
		}
	}()

	return nil
}

func (a *Autopilot) Stop() error {
	return nil
}
