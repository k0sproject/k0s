// Copyright 2022 k0s authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package updates

import (
	"context"
	"strconv"
	"time"

	"github.com/robfig/cron"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	crcli "sigs.k8s.io/controller-runtime/pkg/client"

	apv1beta2 "github.com/k0sproject/k0s/pkg/apis/autopilot.k0sproject.io/v1beta2"
	appc "github.com/k0sproject/k0s/pkg/autopilot/controller/plans/core"
	"github.com/k0sproject/k0s/pkg/autopilot/controller/signal/k0s"
	uc "github.com/k0sproject/k0s/pkg/autopilot/updater"
	k0sinstall "github.com/k0sproject/k0s/pkg/install"
)

const defaultCronSchedule = "@hourly"

type updater struct {
	ctx            context.Context
	cancel         context.CancelFunc
	log            *logrus.Entry
	updateClient   uc.Client
	updateConfig   apv1beta2.UpdateConfig
	k8sClient      crcli.Client
	cron           *cron.Cron
	updateSchedule string
	clusterID      string
	k0sVersion     string
}

var patchOpts []client.PatchOption = []crcli.PatchOption{
	crcli.FieldOwner("autopilot"),
	crcli.ForceOwnership,
}

func newUpdater(parentCtx context.Context, updateConfig apv1beta2.UpdateConfig, k8sClient crcli.Client, clusterID string, updateServerToken string) (*updater, error) {
	updateClient, err := uc.NewClient(updateConfig.Spec.UpdateServer, updateServerToken)
	if err != nil {
		return nil, err
	}

	schedule := updateConfig.Spec.UpgradeStrategy.Cron
	if schedule == "" {
		schedule = defaultCronSchedule
	}

	status, err := k0sinstall.GetStatusInfo(k0s.DefaultK0sStatusSocketPath)
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithCancel(parentCtx)
	u := &updater{
		ctx:            ctx,
		cancel:         cancel,
		log:            logrus.WithField("controller", "update-checker"),
		updateClient:   updateClient,
		updateConfig:   updateConfig,
		updateSchedule: schedule,
		k8sClient:      k8sClient,
		clusterID:      clusterID,
		k0sVersion:     status.Version,
	}

	return u, nil
}

func (u *updater) Run() {
	u.log.Info("running update checker")
	u.cron = cron.New()
	_ = u.cron.AddFunc(u.updateSchedule, u.checkUpdates)
	u.cron.Start()
}

func (u *updater) checkUpdates() {
	u.log.Info("checking updates...")
	var curPlan apv1beta2.Plan
	err := u.k8sClient.Get(u.ctx, crcli.ObjectKey{Name: "autopilot"}, &curPlan)
	if err != nil && !errors.IsNotFound(err) {
		u.log.Errorf("failed to read last plan: %s", err)
		return
	}

	update, err := u.updateClient.GetUpdate(u.updateConfig.Spec.Channel, u.clusterID, curPlan.Status.State.String(), u.k0sVersion)
	if err != nil {
		u.log.Errorf("failed to read available update from update server: %s", err)
		return
	}

	logrus.Infof("Found next version to update to: %s", update.Version)
	if !u.needToUpdate() {
		u.log.Info("no need to update, existing plan has either matching version or in-progress already")
		return
	}

	plan := u.toPlan(update)

	err = u.k8sClient.Patch(u.ctx, &plan, client.Apply, patchOpts...)
	if err != nil {
		u.log.Errorf("failed to patch plan: %s", err)
		return
	}
	u.log.Info("successfully updated plan")
}

func (u *updater) Stop() {
	u.cron.Stop()
	u.cancel()
}

// needToUpdate checks the need to update. we'll create the update Plan if:
// - there's no existing plan
func (u *updater) needToUpdate() bool {
	var plan apv1beta2.Plan
	err := u.k8sClient.Get(u.ctx, crcli.ObjectKey{Name: "autopilot"}, &plan)
	if err != nil && errors.IsNotFound(err) {
		return true
	}

	if plan.Status.State == appc.PlanCompleted {
		return true
	}

	return false
}

func (u *updater) toPlan(nextVersion *uc.Update) apv1beta2.Plan {
	p := apv1beta2.Plan{
		TypeMeta: v1.TypeMeta{
			Kind:       "Plan",
			APIVersion: "autopilot.k0sproject.io/v1beta2",
		},
		ObjectMeta: v1.ObjectMeta{
			Name: "autopilot",
		},
		Spec: u.updateConfig.Spec.PlanSpec,
	}

	platforms := make(apv1beta2.PlanPlatformResourceURLMap)
	for arch, url := range nextVersion.DownloadURLs["k0s"] {
		platforms[arch] = apv1beta2.PlanResourceURL{
			URL: url,
			// TODO: Sha256 of file
		}
	}

	p.Spec.Id = strconv.FormatInt(time.Now().Unix(), 10)
	p.Spec.Timestamp = strconv.FormatInt(time.Now().Unix(), 10)
	p.Spec.Commands = append(p.Spec.Commands, apv1beta2.PlanCommand{
		K0sUpdate: &apv1beta2.PlanCommandK0sUpdate{
			Version:   string(nextVersion.Version),
			Platforms: platforms,
			Targets: apv1beta2.PlanCommandTargets{
				Controllers: apv1beta2.PlanCommandTarget{
					Discovery: apv1beta2.PlanCommandTargetDiscovery{
						Selector: &apv1beta2.PlanCommandTargetDiscoverySelector{},
					},
				},
				Workers: apv1beta2.PlanCommandTarget{
					Discovery: apv1beta2.PlanCommandTargetDiscovery{
						Selector: &apv1beta2.PlanCommandTargetDiscoverySelector{},
					},
				},
			},
			// TODO: Targets
		},
	})

	return p
}
