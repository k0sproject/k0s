//go:build unix

// SPDX-FileCopyrightText: 2022 k0s authors
// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"bytes"
	"context"
	_ "embed"
	"errors"
	"fmt"
	"net/netip"
	"slices"
	"sync"
	"time"

	"github.com/k0sproject/k0s/pkg/applier"
	apcli "github.com/k0sproject/k0s/pkg/autopilot/client"
	apcont "github.com/k0sproject/k0s/pkg/autopilot/controller"
	aproot "github.com/k0sproject/k0s/pkg/autopilot/controller/root"
	"github.com/k0sproject/k0s/pkg/autopilot/controller/updates"
	"github.com/k0sproject/k0s/pkg/component/controller/leaderelector"
	"github.com/k0sproject/k0s/pkg/component/manager"
	"github.com/k0sproject/k0s/pkg/config"
	"github.com/k0sproject/k0s/pkg/kubernetes"
	"github.com/k0sproject/k0s/pkg/leaderelection"
	"github.com/k0sproject/k0s/static"

	"github.com/sirupsen/logrus"
)

const AutopilotStackName = "autopilot"

var _ manager.Component = (*Autopilot)(nil)

//go:embed autopilot.yaml
var autopilotStack []byte

type Autopilot struct {
	K0sVars              *config.CfgVars
	KubeletExtraArgs     string
	APIAddress           netip.Addr
	KubeAPIPort          int
	AdminClientFactory   kubernetes.ClientFactoryInterface
	LeaderElector        leaderelector.Interface
	ClusterInfoCollector *updates.ClusterInfoCollector
	Workloads            bool

	stop func()
}

func (a *Autopilot) Init(context.Context) error {
	return nil
}

func (a *Autopilot) Start(ctx context.Context) error {
	log := logrus.WithFields(logrus.Fields{"component": "autopilot"})

	autopilotClientFactory := &apcli.ClientFactory{
		ClientFactoryInterface: a.AdminClientFactory,
	}

	autopilotRoot, err := apcont.NewRootController(aproot.RootConfig{
		InvocationID:        a.K0sVars.InvocationID,
		KubeConfig:          a.K0sVars.AdminKubeConfigPath,
		K0sDataDir:          a.K0sVars.DataDir,
		KubeletExtraArgs:    a.KubeletExtraArgs,
		KubeAPIPort:         a.KubeAPIPort,
		Mode:                "controller",
		ManagerPort:         8899,
		MetricsBindAddr:     "0",
		HealthProbeBindAddr: "0",
	}, logrus.WithFields(logrus.Fields{"component": "autopilot"}), a.Workloads, autopilotClientFactory, a.ClusterInfoCollector, a.APIAddress)
	if err != nil {
		return fmt.Errorf("failed to create autopilot controller: %w", err)
	}

	ctx, cancel := context.WithCancelCause(context.Background())
	var wg sync.WaitGroup

	wg.Go(func() { a.applyManifests(ctx) })
	wg.Go(func() {
		if err := autopilotRoot.Run(ctx); err != nil {
			log.WithError(err).Error("Error while running autopilot")
		}
	})

	a.stop = func() {
		cancel(errors.New("autopilot controller component is stopping"))
		wg.Wait()
	}

	return nil
}

func (a *Autopilot) Stop() error {
	if stop := a.stop; stop != nil {
		stop()
	}
	return nil
}

func (a *Autopilot) applyManifests(ctx context.Context) {
	leaderelection.RunLeaderTasks(ctx, a.LeaderElector.CurrentStatus, func(ctx context.Context) {
		log := logrus.WithField("component", "autopilot")
		crdResources, err := applier.ReadUnstructuredDir(static.CRDs, AutopilotStackName)
		if err != nil {
			log.WithError(err).Error("Failed to read autopilot CRDs")
			return
		}

		stackResources, err := applier.ReadUnstructuredStream(bytes.NewReader(autopilotStack), AutopilotStackName)
		if err != nil {
			log.WithError(err).Error("Failed to read autopilot stack")
			return
		}

		stack := applier.Stack{
			Name:      AutopilotStackName,
			Resources: slices.Concat(crdResources, stackResources),
			Clients:   a.AdminClientFactory,
		}

		for {
			err := stack.Apply(ctx, true)
			if err == nil {
				return
			}

			select {
			case <-ctx.Done():
				log.WithError(err).Errorf("Failed to apply autopilot stack (%s)", context.Cause(ctx))
			default:
				log.WithError(err).Error("Failed to apply autopilot stack, retrying in 30 seconds")
			}

			select {
			case <-time.After(30 * time.Second):
			case <-ctx.Done():
				log.Infof("Interrupted while waiting for autopilot stack application retry (%s)", context.Cause(ctx))
				return
			}
		}
	})
}
