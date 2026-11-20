// SPDX-FileCopyrightText: 2023 k0s authors
// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	// "github.com/k0sproject/k0s/pkg/component/manager"

	apv1beta2 "github.com/k0sproject/k0s/pkg/apis/autopilot/v1beta2"
	"github.com/k0sproject/k0s/pkg/autopilot/channels"
	"github.com/k0sproject/k0s/pkg/autopilot/controller/updates"
	"github.com/k0sproject/k0s/pkg/build"
	"github.com/k0sproject/k0s/pkg/component/controller/leaderelector"
	"github.com/k0sproject/k0s/pkg/component/manager"
	kubeutil "github.com/k0sproject/k0s/pkg/kubernetes"
	"github.com/k0sproject/k0s/pkg/leaderelection"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/wait"

	"github.com/sirupsen/logrus"
)

// Dummy checks so we catch easily if we miss some interface implementation
var _ manager.Component = (*UpdateProber)(nil)

type UpdateProber struct {
	APClientFactory kubeutil.ClientFactoryInterface
	log             logrus.FieldLogger
	leaderElector   leaderelector.Interface
	collector       *updates.ClusterInfoCollector
	stop            func()
}

func NewUpdateProber(apClientFactory kubeutil.ClientFactoryInterface, leaderElector leaderelector.Interface, collector *updates.ClusterInfoCollector) *UpdateProber {
	return &UpdateProber{
		APClientFactory: apClientFactory,
		log:             logrus.WithFields(logrus.Fields{"component": "updateprober"}),
		leaderElector:   leaderElector,
		collector:       collector,
	}
}

func (u *UpdateProber) Init(ctx context.Context) error {
	return nil
}

func (u *UpdateProber) Start(context.Context) error {
	proberCtx, cancel := context.WithCancelCause(context.Background())
	done := make(chan struct{})
	stopErr := errors.New("update prober is stopping")

	go func() {
		defer close(done)
		u.log.Debug("Checking for updates roughly every 30 minutes")
		leaderelection.RunLeaderTasks(proberCtx, u.leaderElector.CurrentStatus, func(ctx context.Context) {
			for {
				if err := u.checkUpdates(ctx); err != nil && !errors.Is(err, stopErr) && !errors.Is(err, proberCtx.Err()) {
					u.log.WithError(err).Error("Failed to check for updates")
				}

				select {
				// [1600, 2000] seconds, i.e. 30m ± 3m20s
				case <-time.After(wait.Jitter(1600*time.Second, 0.25)):
				case <-ctx.Done():
					return
				}
			}
		})
	}()

	u.stop = func() { cancel(stopErr); <-done }

	return nil
}

func (u *UpdateProber) Stop() error {
	if stop := u.stop; stop != nil {
		stop()
	}
	return nil
}

func (u *UpdateProber) checkUpdates(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()

	if reason, err := u.skipUpdateCheck(ctx); err != nil {
		return err
	} else if reason != "" {
		u.log.WithField("reason", reason).Info("Skipping update check")
		return nil
	}

	u.log.Debug("checking updates")
	// Create new update channel client for default server and latest channel
	// ENV var only to be used for testing purposes
	updateServer := "https://updates.k0sproject.io"
	if os.Getenv("K0S_UPDATE_SERVER") != "" {
		updateServer = os.Getenv("K0S_UPDATE_SERVER")
	}
	u.log.Debugf("using update server: %s", updateServer)
	uc, err := channels.NewChannelClient(updateServer, "latest", "")
	if err != nil {
		return fmt.Errorf("failed to create update channel client: %w", err)
	}

	// Collect cluster info
	var ci *updates.ClusterInfo
	if err := u.retryOnError(ctx, func(err error) bool {
		u.log.WithError(err).Debug("Failed to collect cluster info, retrying after backoff")
		return true
	}, func(ctx context.Context) (err error) {
		ci, err = u.collector.CollectData(ctx)
		return
	}); err != nil {
		return fmt.Errorf("failed to collect cluster info: %w", err)
	}
	extraHeaders := ci.AsMap()
	u.log.Debugf("checking for updates from %s", updateServer)

	// Check for updates
	var v channels.VersionInfo
	if err := u.retryOnError(ctx, func(err error) bool {
		u.log.WithError(err).Debug("Failed to detect latest version, retrying after backoff")
		return true
	}, func(ctx context.Context) (err error) {
		v, err = uc.GetLatest(ctx, extraHeaders)
		return
	}); err != nil {
		return fmt.Errorf("failed to detect latest version: %w", err)
	}
	u.log.Debugf("got latest version: %s", v.Version)

	// Check if current version is outdated
	if isNewer, err := v.IsNewerThan(build.Version); err != nil {
		return fmt.Errorf("failed to compare versions: %w", err)
	} else if !isNewer {
		u.log.Debug("No newer version available")
		return nil
	}

	// Create event to notify admin
	message := "New version available: " + v.Version
	u.log.Info(message)
	name := fmt.Sprintf("k0s-update-probe-%s-%d", v.Version, time.Now().Unix())
	if err := u.retryOnError(ctx, func(err error) bool {
		u.log.WithError(err).Debug("Failed to create event, retrying after backoff")
		return true
	}, func(ctx context.Context) error {
		kc, err := u.APClientFactory.GetClient()
		if err != nil {
			return fmt.Errorf("failed to create k8s client: %w", err)
		}

		e := corev1.Event{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: metav1.NamespaceSystem,
			},
			InvolvedObject: corev1.ObjectReference{
				APIVersion: corev1.SchemeGroupVersion.String(),
				Kind:       "Namespace",
				Name:       metav1.NamespaceSystem,
				Namespace:  metav1.NamespaceSystem,
				UID:        ci.ClusterID,
			},
			Reason:  "NewVersionAvailable",
			Message: message,
			Type:    "Normal",
			Source: corev1.EventSource{
				Component: "k0s",
			},
		}
		_, err = kc.CoreV1().Events(metav1.NamespaceSystem).Create(ctx, &e, metav1.CreateOptions{})
		// A previous attempt may have created the event despite failing.
		if apierrors.IsAlreadyExists(err) {
			err = nil
		}
		return err
	}); err != nil {
		return fmt.Errorf("failed to create event: %w", err)
	}

	return nil
}

func (u *UpdateProber) skipUpdateCheck(ctx context.Context) (string, error) {
	// Check if there's an active UpdateConfig, if there is no need to do this generic polling

	var updateConfigs *metav1.PartialObjectMetadataList
	if err := u.retryOnError(
		ctx,
		func(err error) bool {
			u.log.WithError(err).Debug("Failed to list update configs, retrying after backoff")
			return true
		},
		func(ctx context.Context) error {
			client, err := u.APClientFactory.GetMetadataClient()
			if err != nil {
				return fmt.Errorf("failed to create k8s client: %w", err)
			}
			updateConfigs, err = client.Resource(schema.GroupVersionResource{
				Group:    apv1beta2.GroupVersion.Group,
				Version:  apv1beta2.GroupVersion.Version,
				Resource: "updateconfigs",
			}).List(ctx, metav1.ListOptions{})
			if apierrors.IsNotFound(err) {
				err = nil
			}
			return err
		},
	); err != nil {
		return "", err
	}

	if updateConfigs != nil {
		numUpdateConfigs := len(updateConfigs.Items)
		u.log.Debugf("Found %d update configs", numUpdateConfigs)
		if numUpdateConfigs > 0 {
			return "some Autopilot update configs exist in the cluster", nil
		}
	}

	return "", nil
}

// Attempts to call f seven times, max overall waiting time ~ 32 secs.
func (*UpdateProber) retryOnError(ctx context.Context, retriable func(error) bool, f func(ctx context.Context) error) error {
	for attempt, delay := 1, 500*time.Millisecond; ; attempt, delay = attempt+1, delay*2 {
		err := f(ctx)
		if err == nil || attempt >= 7 || ctx.Err() != nil || !retriable(err) {
			return err
		}

		select {
		case <-time.After(wait.Jitter(delay, 0.1)):
		case <-ctx.Done():
			return fmt.Errorf("%w (last error: %w)", context.Cause(ctx), err)
		}
	}
}
