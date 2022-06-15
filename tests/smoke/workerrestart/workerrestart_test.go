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
package workerrestart

import (
	"context"
	"testing"

	"github.com/stretchr/testify/suite"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/k0sproject/k0s/tests/smoke/common"
)

type WorkerRestartSuite struct {
	common.FootlooseSuite
}

func (s *WorkerRestartSuite) TestK0sWorkerRestart() {
	s.NoError(s.InitController(0))

	kc, err := s.KubeClient(s.ControllerNode(0))
	s.NoError(err)

	s.NoError(s.RunWorkers())
	err = s.WaitForNodeReady(s.WorkerNode(0), kc)
	s.NoError(err)

	// kill the worker
	ssh, err := s.SSH(s.WorkerNode(0))
	s.Require().NoError(err)
	defer ssh.Disconnect()
	s.T().Log("killing k0s")
	_, err = ssh.ExecWithOutput("kill $(pidof k0s) && while pidof k0s; do sleep 0.1s; done")
	s.Require().NoError(err)

	// restart worker and make sure it comes up
	s.T().Logf("Restart worker")
	s.NoError(s.RunWorkers())
	err = s.WaitForNodeReady(s.WorkerNode(0), kc)
	s.NoError(err)

	pods, err := kc.CoreV1().Pods("kube-system").List(context.TODO(), v1.ListOptions{
		Limit: 100,
	})
	s.NoError(err)

	podCount := len(pods.Items)

	s.T().Logf("found %d pods in kube-system", podCount)
	s.Greater(podCount, 0, "expecting to see few pods in kube-system namespace")

	s.T().Log("waiting to see CNI pods ready")
	s.NoError(common.WaitForKubeRouterReady(kc), "CNI did not start")
}

func TestWorkerRestartSuite(t *testing.T) {
	s := WorkerRestartSuite{
		common.FootlooseSuite{
			ControllerCount: 1,
			WorkerCount:     1,
		},
	}
	suite.Run(t, &s)
}
