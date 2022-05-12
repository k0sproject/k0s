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

package clusterconfig

import (
	"context"
	"time"

	cfgClient "github.com/k0sproject/k0s/pkg/apis/k0s.k0sproject.io/clientset/typed/k0s.k0sproject.io/v1beta1"
	"github.com/k0sproject/k0s/pkg/apis/k0s.k0sproject.io/v1beta1"
	"github.com/k0sproject/k0s/pkg/constant"
	kubeutil "github.com/k0sproject/k0s/pkg/kubernetes"
	"github.com/sirupsen/logrus"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ ConfigSource = (*apiConfigSource)(nil)

type apiConfigSource struct {
	configClient cfgClient.ClusterConfigInterface
	resultChan   chan *v1beta1.ClusterConfig

	lastKnownVersion string
}

func NewAPIConfigSource(kubeClientFactory kubeutil.ClientFactoryInterface) (ConfigSource, error) {
	configClient, err := kubeClientFactory.GetConfigClient()
	if err != nil {
		return nil, err
	}
	a := &apiConfigSource{
		configClient: configClient,
		resultChan:   make(chan *v1beta1.ClusterConfig),
	}
	return a, nil
}

func (a *apiConfigSource) Release(ctx context.Context) {
	go func() {
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				err := a.getAndSendConfig(ctx)
				if err != nil {
					logrus.Errorf("failed to source and propagate cluster config: %v", err)
				}
			case <-ctx.Done():
				return
			}
		}
	}()
}

func (a *apiConfigSource) ResultChan() <-chan *v1beta1.ClusterConfig {
	return a.resultChan
}

func (a apiConfigSource) Stop() error {
	if a.resultChan != nil {
		close(a.resultChan)
	}
	return nil
}

func (a *apiConfigSource) NeedToStoreInitialConfig() bool {
	return true
}

func (a *apiConfigSource) getAndSendConfig(ctx context.Context) error {
	cfg, err := a.configClient.Get(ctx, constant.ClusterConfigObjectName, v1.GetOptions{})
	if err != nil {
		return err
	}
	// Push changes only when the config actually changes
	if a.lastKnownVersion == cfg.ResourceVersion {
		return nil
	}
	a.lastKnownVersion = cfg.ResourceVersion
	a.resultChan <- cfg

	return nil
}
