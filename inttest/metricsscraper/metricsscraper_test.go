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

package metricsscraper

import (
	"context"
	"encoding/json"
	"slices"
	"testing"
	"time"

	"github.com/k0sproject/k0s/inttest/common"

	"k8s.io/apimachinery/pkg/util/wait"

	"github.com/stretchr/testify/suite"
)

type MetricsScraperSuite struct {
	common.BootlooseSuite
}

func (s *MetricsScraperSuite) TestK0sGetsUp() {
	s.NoError(s.InitController(0, "--enable-worker", "--enable-metrics-scraper"))

	kc, err := s.KubeClient(s.ControllerNode(0))
	s.Require().NoError(err)

	err = s.WaitForNodeReady(s.ControllerNode(0), kc)
	s.Require().NoError(err)

	s.T().Logf("Waiting to see pushgateway is ready")
	s.Require().NoError(s.waitForPushgateway())

	s.T().Logf("Waiting for metrics")
	s.Require().NoError(s.waitForMetrics("kube-scheduler", "kube-controller-manager", "etcd"))
}

func (s *MetricsScraperSuite) waitForPushgateway() error {
	kc, err := s.KubeClient(s.ControllerNode(0))
	if err != nil {
		return err
	}

	return common.WaitForDeployment(s.Context(), kc, "k0s-pushgateway", "k0s-system")
}

func (s *MetricsScraperSuite) waitForMetrics(expectedJobs ...string) error {
	kc, err := s.KubeClient(s.ControllerNode(0))
	if err != nil {
		return err
	}

	slices.Sort(expectedJobs)

	return wait.PollUntilContextCancel(s.Context(), 5*time.Second, true, func(ctx context.Context) (done bool, err error) {
		b, err := kc.RESTClient().Get().AbsPath("/api/v1/namespaces/k0s-system/services/http:k0s-pushgateway:http/proxy/api/v1/metrics").DoRaw(s.Context())
		if err != nil {
			return false, nil
		}

		var metrics struct {
			Data []struct {
				// Last Unix time when changing this group in the Pushgateway succeeded.
				PushTimeSeconds struct {
					Metrics []struct {
						Labels map[string]string `json:"labels"`
						Value  string            `json:"value"`
					} `json:"metrics"`
				} `json:"push_time_seconds"`
			} `json:"data"`
			Status string `json:"status"`
		}
		if err := json.Unmarshal(b, &metrics); err != nil {
			s.T().Log(err)
			return false, nil
		}

		if metrics.Status != "success" {
			return false, err
		}

		// Collect all the jobs that had successful pushes.
		var jobs []string
		for i := range metrics.Data {
			pts := &metrics.Data[i].PushTimeSeconds
			for i := range pts.Metrics {
				if job, ok := pts.Metrics[i].Labels["job"]; ok {
					if pts.Metrics[i].Value > "0" {
						if idx, found := slices.BinarySearch(jobs, job); !found {
							jobs = slices.Insert(jobs, idx, job)
						}
						break
					}
				}
			}
		}

		s.T().Log("Jobs:", jobs)

		// Return if the job lists match.
		return slices.Equal(expectedJobs, jobs), nil
	})
}

func TestMetricsScraperSuite(t *testing.T) {
	s := MetricsScraperSuite{
		common.BootlooseSuite{
			ControllerCount: 1,
			ControllerUmask: 027,
		},
	}
	suite.Run(t, &s)
}
