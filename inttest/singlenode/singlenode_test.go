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

package singlenode

import (
	"fmt"

	"testing"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/yaml"

	"github.com/stretchr/testify/suite"

	"github.com/k0sproject/k0s/inttest/common"
	"github.com/k0sproject/k0s/pkg/apis/k0s/v1beta1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const k0sPartialConfig = `
spec:
  api:
    sans:
    - 1.2.3.4
`

type SingleNodeSuite struct {
	common.BootlooseSuite
}

func (s *SingleNodeSuite) TestK0sGetsUp() {
	s.PutFile(s.ControllerNode(0), "/tmp/k0s.yaml", k0sPartialConfig)
	s.NoError(s.InitController(0, "--single", "--config=/tmp/k0s.yaml"))

	kc, err := s.KubeClient(s.ControllerNode(0))
	s.Require().NoError(err)

	err = s.WaitForNodeReady(s.ControllerNode(0), kc)
	s.NoError(err)

	s.AssertSomeKubeSystemPods(kc)

	s.T().Log("waiting to see CNI pods ready")
	s.NoError(common.WaitForKubeRouterReady(s.Context(), kc), "CNI did not start")

	s.Run("verify", func() {
		ssh, err := s.SSH(s.Context(), s.ControllerNode(0))
		s.Require().NoError(err, "failed to SSH into controller")
		defer ssh.Disconnect()

		s.Run(("kineIsDefaultStorage"), func() {
			_, err = ssh.ExecWithOutput(s.Context(), "test -e /var/lib/k0s/bin/kine && ps xa | grep kine")
			s.NoError(err)
		})

		s.Run(("noControllerJoinTokens"), func() {
			noToken, err := ssh.ExecWithOutput(s.Context(), fmt.Sprintf("'%s' token create --role=controller", s.K0sFullPath))
			s.Error(err)
			s.Equal("Error: refusing to create token: cannot join into a single node cluster", noToken)
		})

		s.Run(("noWorkerJoinTokens"), func() {
			noToken, err := ssh.ExecWithOutput(s.Context(), fmt.Sprintf("'%s' token create --role=worker", s.K0sFullPath))
			s.Error(err)
			s.Equal("Error: refusing to create token: cannot join into a single node cluster", noToken)
		})

		s.Run("leader election disabled for scheduler", func() {
			_, err := kc.CoordinationV1().Leases("kube-system").Get(s.Context(), "kube-scheduler", v1.GetOptions{})
			s.Error(err)
			s.True(apierrors.IsNotFound(err))
		})

		s.Run("leader election disabled for controller manager", func() {
			_, err := kc.CoordinationV1().Leases("kube-system").Get(s.Context(), "kube-controller-manager", v1.GetOptions{})
			s.Error(err)
			s.True(apierrors.IsNotFound(err))
		})

		// test with etcd backend in config
		s.Run(("killK0s"), func() {
			_, err = ssh.ExecWithOutput(s.Context(), "kill $(pidof k0s) && while pidof k0s; do sleep 0.1s; done")
			s.NoError(err)
		})

		s.PutFile(s.ControllerNode(0), "/tmp/k0s.yaml", k0sConfig)
		s.Require().NoError(err, "failed to upload k0s.yaml")

		s.NoError(s.InitController(0, "--single", "--config=/tmp/k0s.yaml"))

		s.Run(("etcdIsRunning"), func() {
			_, err = ssh.ExecWithOutput(s.Context(), "test -e /var/lib/k0s/bin/etcd && ps xa | grep etcd")
			s.NoError(err)
		})

		s.Run("no kube-bridge address in default config", func() {
			cfg, err := ssh.ExecWithOutput(s.Context(), "k0s config create")
			s.NoError(err)
			config := &v1beta1.ClusterConfig{}
			s.NoError(yaml.Unmarshal([]byte(cfg), config))

			s.NotEqual("10.244.0.1", config.Spec.API.Address)
			s.NotEqual("10.244.0.1", config.Spec.Storage.Etcd.PeerAddress)

		})
	})
}

const k0sConfig = `
spec:
  storage:
    type: etcd
`

func TestSingleNodeSuite(t *testing.T) {
	s := SingleNodeSuite{
		common.BootlooseSuite{
			ControllerCount: 1,
			ControllerUmask: 027,
		},
	}
	suite.Run(t, &s)
}
