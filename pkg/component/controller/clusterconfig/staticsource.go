/*
Copyright 2021 k0s authors

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

	"github.com/k0sproject/k0s/pkg/apis/k0s/v1beta1"
	"github.com/sirupsen/logrus"
)

var _ ConfigSource = (*staticSource)(nil)

type staticSource struct {
	staticConfig *v1beta1.ClusterConfig
	resultChan   chan *v1beta1.ClusterConfig
}

func NewStaticSource(staticConfig *v1beta1.ClusterConfig) ConfigSource {
	return &staticSource{
		staticConfig: staticConfig,
		resultChan:   make(chan *v1beta1.ClusterConfig),
	}
}

// Init implements [manager.Component].
func (*staticSource) Init(context.Context) error { return nil }

// Start implements [manager.Component].
func (s *staticSource) Start(ctx context.Context) error {
	logrus.WithField("component", "static-config-source").Debug("sending static config via channel")
	select {
	case s.resultChan <- s.staticConfig:
		return nil
	case <-ctx.Done():
		return context.Cause(ctx)
	}
}

// ResultChan implements [ConfigSource].
func (s *staticSource) ResultChan() <-chan *v1beta1.ClusterConfig {
	return s.resultChan
}

// Stop implements [manager.Component].
func (s *staticSource) Stop() error {
	close(s.resultChan)
	return nil
}
