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

package metricscraper

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
	corev1 "k8s.io/api/core/v1"

	"github.com/k0sproject/k0s/inttest/common"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
)

type MetricScraperSuite struct {
	common.FootlooseSuite
}

func (s *MetricScraperSuite) TestK0sGetsUp() {
	s.NoError(s.InitController(0, "--single", "--enable-metrics-scraper"))

	kc, err := s.KubeClient(s.ControllerNode(0))
	s.Require().NoError(err)

	err = s.WaitForNodeReady(s.ControllerNode(0), kc)
	s.NoError(err)

	s.NoError(s.waitForPushgateway())

	s.NoError(s.waitForMetrics())
}

func (s *MetricScraperSuite) waitForPushgateway() error {
	s.T().Logf("waiting to see pushgateway is ready")
	kc, err := s.KubeClient(s.ControllerNode(0))
	if err != nil {
		return err
	}

	return wait.PollImmediate(time.Second, 2*time.Minute, func() (done bool, err error) {
		pods, err := kc.CoreV1().Pods("k0s-system").List(s.Context(), v1.ListOptions{})
		if err != nil {
			return false, nil
		}

		for _, pod := range pods.Items {
			if strings.HasPrefix(pod.Name, "k0s-pushgateway") {
				return pods.Items[0].Status.Phase == corev1.PodRunning, nil
			}
		}

		return false, nil
	})
}

func (s *MetricScraperSuite) waitForMetrics() error {
	s.T().Logf("waiting to see metrics")

	kc, err := s.KubeClient(s.ControllerNode(0))
	if err != nil {
		return err
	}

	return wait.PollImmediate(time.Second*5, 2*time.Minute, func() (done bool, err error) {

		b, err := kc.RESTClient().Get().AbsPath("/api/v1/namespaces/k0s-system/services/http:k0s-pushgateway:http/proxy/metrics").DoRaw(s.Context())
		if err != nil {
			return false, nil
		}

		// wait for kube-scheduler and kube-controller-manager metrics
		return strings.Contains(string(b), `job="kube-scheduler"`) && strings.Contains(string(b), `job="kube-controller-manager"`), nil
	})
}

func TestMetricScraperSuite(t *testing.T) {
	s := MetricScraperSuite{
		common.FootlooseSuite{
			ControllerCount: 1,
			ControllerUmask: 027,
		},
	}
	suite.Run(t, &s)
}
