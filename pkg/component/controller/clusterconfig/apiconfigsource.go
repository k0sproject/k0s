// SPDX-FileCopyrightText: 2021 k0s authors
// SPDX-License-Identifier: Apache-2.0

package clusterconfig

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/k0sproject/k0s/internal/sync/value"
	"github.com/k0sproject/k0s/pkg/apis/k0s/v1beta1"
	k0sclient "github.com/k0sproject/k0s/pkg/client/clientset/typed/k0s/v1beta1"
	"github.com/k0sproject/k0s/pkg/constant"
	kubeutil "github.com/k0sproject/k0s/pkg/kubernetes"
	"github.com/k0sproject/k0s/pkg/kubernetes/watch"
	"github.com/k0sproject/k0s/pkg/leaderelection"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"

	"github.com/sirupsen/logrus"
)

var _ ConfigSource = (*apiConfigSource)(nil)

type apiConfigSource struct {
	configClient k0sclient.ClusterConfigInterface
	leaderStatus leaderelection.StatusFunc
	resultChan   chan *v1beta1.ClusterConfig
	stop         func()
}

func NewAPIConfigSource(kubeClientFactory kubeutil.ClientFactoryInterface, leaderStatus leaderelection.StatusFunc) (ConfigSource, error) {
	configClient, err := kubeClientFactory.GetConfigClient()
	if err != nil {
		return nil, err
	}
	a := &apiConfigSource{
		configClient: configClient,
		leaderStatus: leaderStatus,
		resultChan:   make(chan *v1beta1.ClusterConfig),
	}
	return a, nil
}

// Init implements [manager.Component].
func (*apiConfigSource) Init(context.Context) error { return nil }

// Start implements [manager.Component].
func (a *apiConfigSource) Start(context.Context) error {
	var lastObservedVersion string

	log := logrus.WithField("component", "clusterconfig.apiConfigSource")
	watch := watch.ClusterConfigs(a.configClient).
		WithObjectName(constant.ClusterConfigObjectName).
		WithErrorCallback(func(err error) (time.Duration, error) {
			if retryAfter, e := watch.IsRetryable(err); e == nil {
				log.WithError(err).Infof(
					"Transient error while watching for updates to cluster configuration"+
						", last observed version is %q"+
						", starting over after %s ...",
					lastObservedVersion, retryAfter,
				)
				return retryAfter, nil
			}

			retryAfter := 10 * time.Second
			log.WithError(err).Errorf(
				"Failed to watch for updates to cluster configuration"+
					", last observed version is %q"+
					", starting over after %s ...",
				lastObservedVersion, retryAfter,
			)
			return retryAfter, nil
		})

	ctx, cancel := context.WithCancel(context.Background())
	var wg sync.WaitGroup
	a.stop = func() { cancel(); wg.Wait() }

	// TODO: Remove in k0s 1.38+: Sanitize feature gates.
	var sanitizedConfig value.Latest[*v1beta1.ClusterConfig]

	wg.Go(func() {
		defer close(a.resultChan)
		_ = watch.Until(ctx, func(clusterConfig *v1beta1.ClusterConfig) (bool, error) {
			// Push changes only when the config actually changes
			if lastObservedVersion == clusterConfig.ResourceVersion {
				return false, nil
			}

			log.Debugf("Cluster configuration update to resource version %q", clusterConfig.ResourceVersion)
			lastObservedVersion = clusterConfig.ResourceVersion

			if clusterConfig.Spec != nil { // TODO: Remove in k0s 1.38+: Sanitize feature gates.
				if sanitized := clusterConfig.Spec.FeatureGates.Sanitized(); sanitized == nil {
					sanitizedConfig.Set(nil)
				} else {
					log.Info("Sanitized feature gates from ", clusterConfig.Spec.FeatureGates, " to ", sanitized)
					clusterConfig.Spec.FeatureGates = sanitized
					sanitizedConfig.Set(clusterConfig.DeepCopy())
				}
			} else {
				sanitizedConfig.Set(nil)
			}

			select {
			case a.resultChan <- clusterConfig:
			case <-ctx.Done():
			}

			return false, nil
		})
	})

	wg.Go(func() { // TODO: Remove in k0s 1.38+: Sanitize feature gates.
		leaderelection.RunLeaderTasks(ctx, a.leaderStatus, func(ctx context.Context) {
			for {
				config, configChanged := sanitizedConfig.Peek()
				var retry <-chan time.Time

				if config != nil {
					concurrentChange := errors.New("concurrent configuration change")
					ctx, cancel := context.WithCancelCause(ctx)
					go func() {
						select {
						case <-ctx.Done():
						case <-configChanged:
							cancel(concurrentChange)
						}
					}()

					updated, err := a.configClient.Update(ctx, config, metav1.UpdateOptions{})
					cancel(nil)
					if err != nil {
						cause := context.Cause(ctx)
						if !errors.Is(cause, concurrentChange) && !errors.Is(cause, leaderelection.ErrLostLead) {
							log.WithError(err).Errorf("Failed to update sanitized cluster configuration, resource version was %q", config.ResourceVersion)
							if !apierrors.IsConflict(err) {
								retry = time.After(wait.Jitter(50*time.Second, 0.4))
							}
						}
					} else {
						log.Infof("Updated sanitized cluster configuration, new resource version is %q", updated.ResourceVersion)
					}
				}

				select {
				case <-configChanged:
				case <-retry:
				case <-ctx.Done():
					return
				}
			}
		})
	})

	return nil
}

// ResultChan implements [ConfigSource].
func (a *apiConfigSource) ResultChan() <-chan *v1beta1.ClusterConfig {
	return a.resultChan
}

// Stop implements [manager.Component].
func (a *apiConfigSource) Stop() error {
	a.stop()
	return nil
}
