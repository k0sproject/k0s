/*
Copyright 2023 k0s authors

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

package clusterconfig

import (
	"context"
	"time"

	k0sclient "github.com/k0sproject/k0s/pkg/apis/k0s.k0sproject.io/clientset/typed/k0s.k0sproject.io/v1beta1"
	"github.com/k0sproject/k0s/pkg/apis/k0s.k0sproject.io/v1beta1"
	"github.com/k0sproject/k0s/pkg/constant"
	kubeutil "github.com/k0sproject/k0s/pkg/kubernetes"
	"github.com/k0sproject/k0s/pkg/kubernetes/watch"

	"github.com/sirupsen/logrus"
)

var _ ConfigSource = (*apiConfigSource)(nil)

type apiConfigSource struct {
	configClient k0sclient.ClusterConfigInterface
	resultChan   chan *v1beta1.ClusterConfig
}

func NewAPIConfigSource(kubeClientFactory kubeutil.ClientFactoryInterface) (ConfigSource, error) {
	configClient, err := kubeClientFactory.GetConfigClient()
	if err != nil {
		return nil, err
	}
	a := &apiConfigSource{
		configClient: configClient,
		resultChan:   make(chan *v1beta1.ClusterConfig, 1),
	}
	return a, nil
}

func (a *apiConfigSource) Release(ctx context.Context) {
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

	go func() {
		_ = watch.Until(ctx, func(cfg *v1beta1.ClusterConfig) (bool, error) {
			// Push changes only when the config actually changes
			if lastObservedVersion != cfg.ResourceVersion {
				log.Debugf("Cluster configuration update to resource version %q", cfg.ResourceVersion)
				lastObservedVersion = cfg.ResourceVersion
				a.resultChan <- cfg
			}
			return false, nil
		})
	}()
}

func (a *apiConfigSource) ResultChan() <-chan *v1beta1.ClusterConfig {
	return a.resultChan
}

func (a apiConfigSource) Stop() {
	if a.resultChan != nil {
		close(a.resultChan)
	}
}

func (a *apiConfigSource) NeedToStoreInitialConfig() bool {
	return true
}
