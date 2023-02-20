/*
Copyright 2020 k0s authors

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
	"fmt"
	"testing"

	"github.com/avast/retry-go"
	"github.com/stretchr/testify/suite"
	"github.com/weaveworks/footloose/pkg/config"

	"github.com/k0sproject/k0s/inttest/common"
)

type BYOCRISuite struct {
	common.FootlooseSuite
}

func (s *BYOCRISuite) TestK0sGetsUp() {

	s.NoError(s.InitController(0))
	s.Require().NoError(s.runDockerWorker())

	kc, err := s.KubeClient(s.ControllerNode(0))
	s.Require().NoError(err)

	err = s.WaitForNodeReady(s.WorkerNode(0), kc)
	s.NoError(err)

	s.AssertSomeKubeSystemPods(kc)

	s.T().Log("waiting to see CNI pods ready")
	s.NoError(common.WaitForKubeRouterReady(s.Context(), kc), "CNI did not start")
}

func (s *BYOCRISuite) runDockerWorker() error {
	token, err := s.GetJoinToken("worker")
	if err != nil {
		return err
	}
	if token == "" {
		return fmt.Errorf("got empty token for worker join")
	}
	sshWorker, err := s.SSH(s.Context(), s.WorkerNode(0))
	if err != nil {
		return err
	}
	defer sshWorker.Disconnect()

	_, err = sshWorker.ExecWithOutput(s.Context(), "apk add docker && rc-service docker start")
	if err != nil {
		return err
	}
	// We need to also start the cri-dockerd as the shim is no longer bundled with kubelet codebase
	_, err = sshWorker.ExecWithOutput(s.Context(), "rc-service cri-dockerd start")
	if err != nil {
		return err
	}

	s.T().Log("Waiting for cri-dockerd to start up")

	s.Require().NoError(retry.Do(
		func() error {
			_, err = sshWorker.ExecWithOutput(s.Context(), "[ -e /var/run/cri-dockerd.sock ]")
			return err
		},
		retry.LastErrorOnly(true),
		retry.Context(s.Context()),
	), "The socket file for cri-dockerd doesn't exist. Is it running?")

	workerCommand := fmt.Sprintf(`nohup /usr/local/bin/k0s worker --debug --cri-socket remote:unix:///var/run/cri-dockerd.sock "%s" >/tmp/k0s-worker.log 2>&1 &`, token)
	_, err = sshWorker.ExecWithOutput(s.Context(), workerCommand)
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
