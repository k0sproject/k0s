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

package basic

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/suite"
	"golang.org/x/sync/errgroup"

	"github.com/k0sproject/k0s/inttest/common"
)

type UpgradeSuite struct {
	common.FootlooseSuite
}

const previousVersion = "v1.24.4+k0s.0"

func (s *UpgradeSuite) TestK0sGetsUp() {
	dlCommand := fmt.Sprintf("curl -sSfL https://get.k0s.sh | K0S_VERSION=%s sh", previousVersion)
	g := errgroup.Group{}
	g.Go(func() error {
		ssh, err := s.SSH(s.Context(), s.ControllerNode(0))
		if err != nil {
			return err
		}
		defer ssh.Disconnect()
		_, err = ssh.ExecWithOutput(s.Context(), dlCommand)
		if err != nil {
			return err
		}
		_, err = ssh.ExecWithOutput(s.Context(), "/usr/local/bin/k0s install controller")
		if err != nil {
			return err
		}
		_, err = ssh.ExecWithOutput(s.Context(), "/usr/local/bin/k0s start")
		if err != nil {
			return err
		}
		return nil
	})

	for i := 0; i < s.WorkerCount; i++ {
		node := s.WorkerNode(i)
		g.Go(func() error {
			ssh, err := s.SSH(s.Context(), node)
			if err != nil {
				return err
			}
			defer ssh.Disconnect()
			_, err = ssh.ExecWithOutput(s.Context(), dlCommand)
			if err != nil {
				return err
			}
			return nil
		})
	}

	s.Require().NoError(g.Wait())

	// use the oldVersion k0s for footloose operations
	s.K0sFullPath = "/usr/local/bin/k0s"

	s.Require().NoError(s.WaitForKubeAPI(s.ControllerNode(0)))
	token, err := s.GetJoinToken("worker")
	s.Require().NoError(err)

	for i := 0; i < s.WorkerCount; i++ {
		node := s.WorkerNode(i)
		g.Go(func() error {
			ssh, err := s.SSH(s.Context(), node)
			if err != nil {
				return err
			}
			defer ssh.Disconnect()
			s.PutFile(node, "/etc/k0s.token", token)
			_, err = ssh.ExecWithOutput(s.Context(), "/usr/local/bin/k0s install worker --token-file /etc/k0s.token")
			if err != nil {
				return err
			}
			// plain "k0s start" does not seem to work on open-rc
			_, err = ssh.ExecWithOutput(s.Context(), "service k0sworker start")
			if err != nil {
				return err
			}
			return nil
		})
	}
	s.Require().NoError(g.Wait())

	kc, err := s.KubeClient(s.ControllerNode(0))
	s.Require().NoError(err)

	err = s.WaitForNodeReady(s.WorkerNode(0), kc)
	s.NoError(err)

	err = s.WaitForNodeReady(s.WorkerNode(1), kc)
	s.NoError(err)

	s.AssertSomeKubeSystemPods(kc)

	s.T().Log("waiting to see kube-router pods ready")
	s.NoError(common.WaitForKubeRouterReady(s.Context(), kc), "kube-router did not start")

	// Prev version gets up, let's upgrade everything
	// Upgrade is just swapping the bin and restarting
	for i := 0; i < s.ControllerCount; i++ {
		node := s.ControllerNode(i)
		g.Go(func() error {
			ssh, err := s.SSH(s.Context(), node)
			if err != nil {
				return err
			}
			defer ssh.Disconnect()
			_, err = ssh.ExecWithOutput(s.Context(), "cp -f /dist/k0s /usr/local/bin/k0s")
			if err != nil {
				return err
			}
			_, err = ssh.ExecWithOutput(s.Context(), "service k0scontroller restart")
			if err != nil {
				return err
			}
			return nil
		})
	}
	for i := 0; i < s.WorkerCount; i++ {
		node := s.WorkerNode(i)
		g.Go(func() error {
			ssh, err := s.SSH(s.Context(), node)
			if err != nil {
				return err
			}
			defer ssh.Disconnect()
			_, err = ssh.ExecWithOutput(s.Context(), "cp -f /dist/k0s /usr/local/bin/k0s")
			if err != nil {
				return err
			}
			_, err = ssh.ExecWithOutput(s.Context(), "service k0sworker restart")
			if err != nil {
				return err
			}
			return nil
		})
	}

	s.Require().NoError(g.Wait())
	s.Require().NoError(s.WaitForKubeAPI(s.ControllerNode(0)))
	err = s.WaitForNodeReady(s.WorkerNode(0), kc)
	s.NoError(err)

	err = s.WaitForNodeReady(s.WorkerNode(1), kc)
	s.NoError(err)
}

func TestUpgradeSuite(t *testing.T) {
	s := UpgradeSuite{
		common.FootlooseSuite{
			ControllerCount: 1,
			WorkerCount:     2,
			LaunchMode:      common.LaunchModeOpenRC,
		},
	}
	suite.Run(t, &s)
}
