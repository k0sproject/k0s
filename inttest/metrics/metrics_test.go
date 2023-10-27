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

package metrics

import (
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/k0sproject/k0s/inttest/common"
)

type MetricsSuite struct {
	common.BootlooseSuite
}

func (s *MetricsSuite) TestK0sGetsUp() {
	s.NoError(s.InitController(0))
	s.NoError(s.RunWorkers())

	kc, err := s.KubeClient(s.ControllerNode(0))
	s.Require().NoError(err)

	err = s.WaitForNodeReady(s.WorkerNode(0), kc)
	s.NoError(err)

	cfg, err := s.GetKubeConfig(s.ControllerNode(0))
	s.Require().NoError(err)
	s.T().Log("waiting to see metrics ready")
	s.Require().NoError(common.WaitForMetricsReady(s.Context(), cfg))
}

func TestMetricsSuite(t *testing.T) {
	s := MetricsSuite{
		common.BootlooseSuite{
			ControllerCount: 1,
			WorkerCount:     1,
		},
	}
	suite.Run(t, &s)
}
