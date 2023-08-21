// Copyright 2023 k0s authors
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

package controller

import (
	"context"
	"fmt"
	"os"
	"time"

	// "github.com/k0sproject/k0s/pkg/component/manager"

	"github.com/k0sproject/k0s/pkg/apis/k0s/v1beta1"
	"github.com/k0sproject/k0s/pkg/autopilot/channels"
	apcli "github.com/k0sproject/k0s/pkg/autopilot/client"
	"github.com/k0sproject/k0s/pkg/autopilot/controller/updates"
	"github.com/k0sproject/k0s/pkg/build"
	"github.com/k0sproject/k0s/pkg/component/controller/leaderelector"
	"github.com/k0sproject/k0s/pkg/component/manager"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Dummy checks so we catch easily if we miss some interface implementation
var _ manager.Component = (*UpdateProber)(nil)

type UpdateProber struct {
	APClientFactory apcli.FactoryInterface
	ClusterConfig   *v1beta1.ClusterConfig
	log             logrus.FieldLogger
	leaderElector   leaderelector.Interface
}

func NewUpdateProber(apClientFactory apcli.FactoryInterface, leaderElector leaderelector.Interface) *UpdateProber {
	return &UpdateProber{
		APClientFactory: apClientFactory,
		log:             logrus.WithFields(logrus.Fields{"component": "updateprober"}),
		leaderElector:   leaderElector,
	}
}

func (u *UpdateProber) Init(ctx context.Context) error {
	return nil
}

func (u *UpdateProber) Start(ctx context.Context) error {
	u.log.Debug("starting up")
	// Check for updates in 30min intervals from default update server
	// ENV var only to be used for testing purposes
	updateCheckInterval := time.Duration(30 * time.Minute)
	if os.Getenv("K0S_UPDATE_CHECK_INTERVAL") != "" {
		d, err := time.ParseDuration(os.Getenv("K0S_UPDATE_CHECK_INTERVAL"))
		if err != nil {
			u.log.Warnf("failed to parse K0S_UPDATE_CHECK_INTERVAL, using default value of 30mins: %s", err.Error())
		} else {
			updateCheckInterval = d
		}
	}
	u.log.Debugf("using interval %s", updateCheckInterval.String())
	go func() {
		ticker := time.NewTicker(updateCheckInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				u.checkUpdates(ctx)
			case <-ctx.Done():
				return
			}
		}
	}()
	return nil
}

func (u *UpdateProber) Stop() error {
	return nil
}

func (u *UpdateProber) checkUpdates(ctx context.Context) {
	if !u.leaderElector.IsLeader() {
		u.log.Debug("not leader, skipping check")
	}
	u.log.Debug("checking updates")
	// Check if there's an active UpdateConfig, if there is no need to do this generic polling
	apClient, err := u.APClientFactory.GetAutopilotClient()
	if err != nil {
		u.log.Warnf("failed to create k8s client: %s", err.Error())
	}
	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()

	u.log.Debug("checking if there's existing UpdateConfig objects")
	updateConfigs, err := apClient.AutopilotV1beta2().UpdateConfigs().List(ctx, metav1.ListOptions{})
	if err != nil {
		u.log.Warnf("failed to list update configs: %s", err.Error())
		return
	}
	u.log.Debugf("found %d UpdateConfig objects", len(updateConfigs.Items))
	if len(updateConfigs.Items) > 0 {
		u.log.Debugf("found %d update configs, skipping generic update check", len(updateConfigs.Items))
		return
	}

	// Create new update channel client for default server and latest channel
	// ENV var only to be used for testing purposes
	updateServer := "https://updates.k0sproject.io"
	if os.Getenv("K0S_UPDATE_SERVER") != "" {
		updateServer = os.Getenv("K0S_UPDATE_SERVER")
	}
	u.log.Debugf("using update server: %s", updateServer)
	uc, err := channels.NewChannelClient(updateServer, "latest", "")
	if err != nil {
		u.log.Errorf("failed to create update channel client: %s", err.Error())
		return
	}

	kc, err := u.APClientFactory.GetClient()
	if err != nil {
		u.log.Errorf("failed to create k8s client: %s", err.Error())
		return
	}

	// Collect cluster info
	ci, err := updates.CollectData(ctx, kc)
	if err != nil {
		u.log.Errorf("failed to collect cluster info: %s", err.Error())
		return
	}
	extraHeaders := ci.AsMap()
	u.log.Debugf("checking for updates from %s", updateServer)

	// Check for updates
	v, err := uc.GetLatest(ctx, extraHeaders)
	if err != nil {
		u.log.Errorf("failed to get latest version: %s", err.Error())
		return
	}
	u.log.Debugf("got latest version: %s", v.Version)
	ksns, err := kc.CoreV1().Namespaces().Get(ctx, "kube-system", metav1.GetOptions{})
	if err != nil {
		u.log.WithError(err).Warn("failed to get kube-system namespace details")
	}
	// Check if current version is outdated
	isNewer, err := v.IsNewerThan(build.Version)
	if err != nil {
		u.log.Errorf("failed to compare versions: %s", err.Error())
		return
	}
	if isNewer {
		// Create event to notify admin
		u.log.Infof("New version available: %s", v.Version)
		name := fmt.Sprintf("k0s-update-probe-%s-%d", v.Version, time.Now().Unix())
		e := corev1.Event{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: "kube-system",
			},
			InvolvedObject: corev1.ObjectReference{
				Kind:       "Namespace",
				Name:       "kube-system",
				Namespace:  "kube-system",
				APIVersion: ksns.APIVersion,
				UID:        ksns.UID,
			},
			Reason:  "NewVersionAvailable",
			Message: "New version available: " + v.Version,
			Type:    "Normal",
			Source: corev1.EventSource{
				Component: "k0s",
			},
		}
		if _, err := kc.CoreV1().Events("kube-system").Create(ctx, &e, metav1.CreateOptions{}); err != nil {
			u.log.Errorf("failed to create event: %s", err.Error())
			return
		}
	} else {
		u.log.Debugf("no newer version availablen")
	}
}
