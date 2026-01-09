//go:build unix

// SPDX-FileCopyrightText: 2023 k0s authors
// SPDX-License-Identifier: Apache-2.0

package updates

import (
	"context"
	"os"
	"time"

	apv1beta2 "github.com/k0sproject/k0s/pkg/apis/autopilot/v1beta2"
	uc "github.com/k0sproject/k0s/pkg/autopilot/channels"
	apcli "github.com/k0sproject/k0s/pkg/autopilot/client"
	apcore "github.com/k0sproject/k0s/pkg/autopilot/controller/plans/core"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	"github.com/k0sproject/version"
	"github.com/sirupsen/logrus"
	crcli "sigs.k8s.io/controller-runtime/pkg/client"
)

type periodicUpdater struct {
	ctx             context.Context
	log             *logrus.Entry
	updateConfig    apv1beta2.UpdateConfig
	k8sClient       crcli.Client
	apClientFactory apcli.FactoryInterface

	currentK0sVersion string

	ticker *time.Ticker
}

func newPeriodicUpdater(ctx context.Context, updateConfig apv1beta2.UpdateConfig, k8sClient crcli.Client, apClientFactory apcli.FactoryInterface, currentK0sVersion string) *periodicUpdater {
	return &periodicUpdater{
		ctx:               ctx,
		log:               logrus.WithField("component", "periodic-updater"),
		updateConfig:      updateConfig,
		k8sClient:         k8sClient,
		currentK0sVersion: currentK0sVersion,
		apClientFactory:   apClientFactory,
	}
}

func (u *periodicUpdater) Config() *apv1beta2.UpdateConfig {
	return &u.updateConfig
}

func (u *periodicUpdater) Run() error {
	u.log.Debug("starting periodic updater")
	checkDuration := 10 * time.Minute
	// ENV var used only for testing purposes
	if e := os.Getenv("K0S_UPDATE_PERIOD"); e != "" {
		cd, err := time.ParseDuration(e)
		if err != nil {
			u.log.Errorf("failed to parse %s as duration for update checks: %s", e, err.Error())
		} else {
			checkDuration = cd
		}
	}
	u.log.Debugf("using %s for update check period", checkDuration.String())
	go func() {
		// Check for update every checkDuration, return when context is canceled
		ticker := time.NewTicker(checkDuration)
		u.ticker = ticker
		defer ticker.Stop()
		for {
			select {
			case <-u.ctx.Done():
				u.log.Infof("parent context done, stopping polling")
				return
			case <-ticker.C:
				u.checkForUpdate()
			}
		}
	}()

	return nil
}

func (u *periodicUpdater) Stop() {
	// u.cancel()
	if u.ticker != nil {
		u.ticker.Stop()
	}
}

func (u *periodicUpdater) checkForUpdate() {
	u.log.Debug("checking for updates")
	ctx, cancel := context.WithTimeout(u.ctx, 2*time.Minute)
	defer cancel()

	// Check if there's a token configured
	var token string
	tokenSecret := &corev1.Secret{}
	if err := u.k8sClient.Get(ctx, crcli.ObjectKey{Name: "update-server-token", Namespace: metav1.NamespaceSystem}, tokenSecret); err != nil {
		u.log.Infof("unable to get update server token: %v", err)
	} else {
		token = string(tokenSecret.Data["token"])
	}

	// Fetch the latest version from the update server
	channelClient, err := uc.NewChannelClient(u.updateConfig.Spec.UpdateServer, u.updateConfig.Spec.Channel, token)
	if err != nil {
		u.log.Errorf("failed to create channel client: %v", err)
		return
	}

	k8sClient, err := u.apClientFactory.GetClient()
	if err != nil {
		u.log.Errorf("failed to create k8s client: %v", err)
		return
	}
	// Collect cluster info
	ci, err := CollectData(ctx, k8sClient)
	if err != nil {
		u.log.Errorf("failed to collect cluster info: %s", err.Error())
		return
	}
	extraHeaders := ci.AsMap()

	latestVersion, err := channelClient.GetLatest(ctx, extraHeaders)
	if err != nil {
		u.log.Errorf("failed to get latest version: %v", err)
		return
	}
	u.log.Debugf("got new version: %s", latestVersion.Version)
	// Check if the latest version is newer than the current version
	current, err := version.NewVersion(u.currentK0sVersion)
	if err != nil {
		u.log.Errorf("failed to parse current version: %v", err)
		return
	}
	new, err := version.NewVersion(latestVersion.Version)
	if err != nil {
		u.log.Errorf("failed to parse latest version: %v", err)
		return
	}

	if !new.GreaterThan(current) {
		u.log.Infof("no new version available")
		return
	}

	if !u.updateConfig.Spec.UpgradeStrategy.Periodic.IsWithinPeriod(time.Now()) {
		u.log.Infof("new version available but not within update window")
		return
	}

	u.log.Infof("new version available: %+v", latestVersion)
	// Check if there's existing plan in-progress
	existingPlan := &apv1beta2.Plan{}
	found := true
	if err := u.k8sClient.Get(ctx, types.NamespacedName{Name: "autopilot"}, existingPlan, &crcli.GetOptions{}); err != nil {
		if !errors.IsNotFound(err) {
			u.log.WithError(err).Errorf("failed to get possible existing plans")
			return
		}
		found = false
	}

	if found && existingPlan.Status.State != apcore.PlanCompleted {
		u.log.Infof("existing plan in state %s, won't create a new one", existingPlan.Status.State.String())
		return
	}

	// Create the update plan
	plan := u.updateConfig.ToPlan(latestVersion)
	if err := u.k8sClient.Patch(ctx, &plan, crcli.Apply, patchOpts...); err != nil {
		u.log.Errorf("failed to patch plan: %v", err)
		return
	}
	u.log.Info("successfully updated plan")
}
