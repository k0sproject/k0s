//go:build unix

// SPDX-FileCopyrightText: 2022 k0s authors
// SPDX-License-Identifier: Apache-2.0

package worker

import (
	"context"
	"errors"
	"fmt"
	"time"

	apcli "github.com/k0sproject/k0s/pkg/autopilot/client"
	apcont "github.com/k0sproject/k0s/pkg/autopilot/controller"
	aproot "github.com/k0sproject/k0s/pkg/autopilot/controller/root"
	"github.com/k0sproject/k0s/pkg/component/manager"
	"github.com/k0sproject/k0s/pkg/config"
	"github.com/k0sproject/k0s/pkg/kubernetes"
	"github.com/sirupsen/logrus"

	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/rest"
)

const (
	defaultPollDuration = 5 * time.Second
	defaultPollTimeout  = 5 * time.Minute
)

var _ manager.Component = (*Autopilot)(nil)

type Autopilot struct {
	K0sVars     *config.CfgVars
	CertManager *CertificateManager
}

func (a *Autopilot) Init(ctx context.Context) error {
	return nil
}

func (a *Autopilot) Start(ctx context.Context) error {
	log := logrus.WithFields(logrus.Fields{"component": "autopilot"})

	// Wait 5 mins till we see kubelet auth config in place
	timeout, cancel := context.WithTimeout(ctx, defaultPollTimeout)
	defer cancel()

	var restConfig *rest.Config
	// wait.PollUntilWithContext passes it is own ctx argument as a ctx to the given function
	// Poll until the kubelet config can be loaded successfully, as this is the access to the kube api
	// needed by autopilot.
	if err := wait.PollUntilWithContext(timeout, defaultPollDuration, func(ctx context.Context) (done bool, err error) {
		log.Debugf("Attempting to load autopilot client config")
		if restConfig, err = a.CertManager.GetRestConfig(ctx); err != nil {
			log.WithError(err).Warnf("Failed to load autopilot client config, retrying in %v", defaultPollDuration)
			return false, nil
		}

		return true, nil
	}); err != nil {
		return fmt.Errorf("unable to create autopilot client: %w", err)
	}

	// Without the config, there is nothing that we can do.

	if restConfig == nil {
		return errors.New("unable to create an autopilot client -- timed out")
	}

	autopilotClientFactory := &apcli.ClientFactory{ClientFactoryInterface: &kubernetes.ClientFactory{
		LoadRESTConfig: func() (*rest.Config, error) { return restConfig, nil },
	}}

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
func (a *Autopilot) Stop() error {
	return nil
}
