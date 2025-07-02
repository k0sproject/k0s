// SPDX-FileCopyrightText: 2021 k0s authors
// SPDX-License-Identifier: Apache-2.0

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
