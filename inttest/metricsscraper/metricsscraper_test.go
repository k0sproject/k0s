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

package metricsscraper

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"

	"github.com/k0sproject/k0s/inttest/common"
	"k8s.io/apimachinery/pkg/util/wait"
)

type MetricsScraperSuite struct {
	common.FootlooseSuite
}

func (s *MetricsScraperSuite) TestK0sGetsUp() {
	s.NoError(s.InitController(0, "--single", "--enable-metrics-scraper"))

	kc, err := s.KubeClient(s.ControllerNode(0))
	s.Require().NoError(err)

	err = s.WaitForNodeReady(s.ControllerNode(0), kc)
	s.Require().NoError(err)

	s.T().Logf("Waiting to see pushgateway is ready")
	s.Require().NoError(s.waitForPushgateway())

	s.T().Logf("Waiting for metrics")
	s.Require().NoError(s.waitForMetrics())
}

func (s *MetricsScraperSuite) waitForPushgateway() error {
	kc, err := s.KubeClient(s.ControllerNode(0))
	if err != nil {
		return err
	}

	return common.WaitForDeployment(s.Context(), kc, "k0s-pushgateway", "k0s-system")
}

func (s *MetricsScraperSuite) waitForMetrics() error {
	kc, err := s.KubeClient(s.ControllerNode(0))
	if err != nil {
		return err
	}

	return wait.PollImmediateUntilWithContext(s.Context(), 5*time.Second, func(ctx context.Context) (done bool, err error) {

		b, err := kc.RESTClient().Get().AbsPath("/api/v1/namespaces/k0s-system/services/http:k0s-pushgateway:http/proxy/metrics").DoRaw(s.Context())
		if err != nil {
			return false, nil
		}

		// wait for kube-scheduler and kube-controller-manager metrics
		return strings.Contains(string(b), `job="kube-scheduler"`) && strings.Contains(string(b), `job="kube-controller-manager"`), nil
	})
}

func TestMetricsScraperSuite(t *testing.T) {
	s := MetricsScraperSuite{
		common.FootlooseSuite{
			ControllerCount: 1,
			ControllerUmask: 027,
		},
	}
	suite.Run(t, &s)
}
