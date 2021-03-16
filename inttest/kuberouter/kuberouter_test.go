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
package kuberouter

import (
	"context"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/k0sproject/k0s/inttest/common"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type KubeRouterSuite struct {
	common.FootlooseSuite
}

func (s *KubeRouterSuite) TestK0sGetsUp() {
	s.PutFile("/tmp/k0s.yaml", k0sConfig, "controller0")
	s.Require().NoError(s.InitMainController([]string{"--config=/tmp/k0s.yaml"}))
	s.Require().NoError(s.RunWorkers("/var/lib/k0s"))

	kc, err := s.KubeClient("controller0", "")
	s.NoError(err)

	err = s.WaitForNodeReady("worker0", kc)
	s.NoError(err)

	err = s.WaitForNodeReady("worker1", kc)
	s.NoError(err)

	pods, err := kc.CoreV1().Pods("kube-system").List(context.TODO(), v1.ListOptions{
		Limit: 100,
	})
	s.NoError(err)

	podCount := len(pods.Items)

	s.T().Logf("found %d pods in kube-system", podCount)
	s.Greater(podCount, 0, "expecting to see few pods in kube-system namespace")

	// s.T().Log("waiting to see calico pods ready")
	// s.NoError(common.WaitForCalicoReady(kc), "calico did not start")
}

func TestKubeRouterSuite(t *testing.T) {
	s := KubeRouterSuite{
		common.FootlooseSuite{
			ControllerCount: 1,
			WorkerCount:     2,
		},
	}
	suite.Run(t, &s)
}

const k0sConfig = `
spec:
  network:
    provider: kuberouter
    kuberouter:
`
