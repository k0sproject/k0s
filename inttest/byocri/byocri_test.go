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
package byocri

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/suite"
	"github.com/weaveworks/footloose/pkg/config"

	"github.com/k0sproject/k0s/inttest/common"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type BYOCRISuite struct {
	common.FootlooseSuite
}

func (s *BYOCRISuite) TestK0sGetsUp() {

	s.NoError(s.InitController(0))
	s.Require().NoError(s.runDockerWorker())

	kc, err := s.KubeClient(s.ControllerNode(0))
	s.NoError(err)

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

func (s *BYOCRISuite) runDockerWorker() error {
	token, err := s.GetJoinToken("worker")
	if err != nil {
		return err
	}
	if token == "" {
		return fmt.Errorf("got empty token for worker join")
	}
	sshWorker, err := s.SSH(s.WorkerNode(0))
	if err != nil {
		return err
	}
	defer sshWorker.Disconnect()

	_, err = sshWorker.ExecWithOutput("apk add docker && rc-service docker start")
	if err != nil {
		return err
	}
	// We need to also start the cri-dockerd as the shim is no longer bundled with kubelet codebase
	_, err = sshWorker.ExecWithOutput("rc-service cri-dockerd start")
	if err != nil {
		return err
	}

	workerCommand := fmt.Sprintf(`nohup /usr/local/bin/k0s --debug worker --cri-socket remote:unix:///var/run/cri-dockerd.sock "%s" >/tmp/k0s-worker.log 2>&1 &`, token)
	_, err = sshWorker.ExecWithOutput(workerCommand)
	if err != nil {
		return err
	}

	return nil
}

func TestBYOCRISuite(t *testing.T) {
	s := BYOCRISuite{
		common.FootlooseSuite{
			ControllerCount: 1,
			WorkerCount:     1,
			ExtraVolumes: []config.Volume{
				{
					Type:        "volume",
					Destination: "/var/lib/docker",
				},
			},
		},
	}
	suite.Run(t, &s)
}
