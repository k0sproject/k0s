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
package worker

import (
	"context"
	"errors"
	"fmt"
	"time"

	apcli "github.com/k0sproject/k0s/pkg/autopilot/client"
	apcont "github.com/k0sproject/k0s/pkg/autopilot/controller"
	aproot "github.com/k0sproject/k0s/pkg/autopilot/controller/root"
	"github.com/sirupsen/logrus"

	"github.com/k0sproject/k0s/pkg/component"
	"github.com/k0sproject/k0s/pkg/constant"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/rest"
)

const (
	defaultPollDuration = 5 * time.Second
	defaultPollTimeout  = 5 * time.Minute
)

var _ component.Component = (*Autopilot)(nil)

type Autopilot struct {
	K0sVars constant.CfgVars
}

func (a *Autopilot) Init(ctx context.Context) error {
	return nil
}

func (a *Autopilot) Run(ctx context.Context) error {
	log := logrus.WithFields(logrus.Fields{"component": "autopilot"})

	// Wait 5 mins till we see kubelet auth config in place
	timeout, cancel := context.WithTimeout(ctx, defaultPollTimeout)
	defer cancel()

	// Poll until the kubelet config can be loaded successfully, as this is the access to the kube api
	// needed by autopilot.

	var restConfig *rest.Config
	wait.PollUntilWithContext(timeout, defaultPollDuration, func(ctx context.Context) (done bool, err error) {
		log.Debugf("Attempting to load autopilot client config")
		if restConfig, err = GetRestConfig(ctx, a.K0sVars.KubeletAuthConfigPath); err != nil {
			log.WithError(err).Warnf("Failed to load autopilot client config, retrying in %v", defaultPollDuration)
			return false, nil
		}

		return true, nil
	})

	// Without the config, there is nothing that we can do.

	if restConfig == nil {
		return errors.New("unable to create an autopilot client -- timed out")
	}

	autopilotClientFactory, err := apcli.NewClientFactory(restConfig)
	if err != nil {
		return fmt.Errorf("creating autopilot client factory error: %w", err)
	}

	log.Info("Autopilot client factory created, booting up worker root controller")
	autopilotRoot, err := apcont.NewRootWorker(aproot.RootConfig{
		KubeConfig:          a.K0sVars.KubeletAuthConfigPath,
		K0sDataDir:          a.K0sVars.DataDir,
		Mode:                "worker",
		ManagerPort:         8899,
		MetricsBindAddr:     "0",
		HealthProbeBindAddr: "0",
	}, log, autopilotClientFactory)
	if err != nil {
		return fmt.Errorf("failed to create autopilot worker: %w", err)
	}

	go func() {
		if err := autopilotRoot.Run(ctx); err != nil {
			logrus.WithError(err).Error("Error running autopilot")

			// TODO: We now have a service with nothing running.. now what?
		}
	}()

	return nil
}

// Stop stops Autopilot
func (c *Autopilot) Stop() error {
	return nil
}

// Health-check interface
func (a *Autopilot) Healthy() error { return nil }
