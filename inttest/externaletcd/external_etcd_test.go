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

package externaletcd

import (
	"context"
	"fmt"
	"testing"

	"github.com/avast/retry-go"
	"github.com/k0sproject/k0s/inttest/common"
	"github.com/stretchr/testify/suite"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const k0sConfig = `
spec:
  storage:
    type: etcd
    etcd:
      externalCluster:
        endpoints:
        - http://etcd0:2379
        etcdPrefix: k0s-tenant
`

type ExternalEtcdSuite struct {
	common.FootlooseSuite
}

func (s *ExternalEtcdSuite) TestK0sWithExternalEtcdCluster() {
	s.T().Log("starting etcd")
	err := retry.Do(func() error {
		ssh, err := s.SSH(s.ExternalEtcd(0))
		if err != nil {
			return err
		}
		defer ssh.Disconnect()

		_, err = ssh.ExecWithOutput(
			"ETCD_ADVERTISE_CLIENT_URLS=\"http://etcd0:2379\" " +
				"ETCD_LISTEN_CLIENT_URLS=\"http://0.0.0.0:2379\" " +
				"/opt/etcd > /var/log/etcd.log 2>&1 &")
		return err
	})
	s.Require().NoError(err)

	s.T().Log("configuring k0s controller to resolve etcd0 hostname")
	k0sControllerSSH, err := s.SSH(s.ControllerNode(0))
	s.Require().NoError(err)
	defer k0sControllerSSH.Disconnect()

	_, err = k0sControllerSSH.ExecWithOutput(fmt.Sprintf("echo '%s etcd0' >> /etc/hosts", s.GetExternalEtcdIPAddress()))
	s.Require().NoError(err)

	s.T().Log("starting k0s controller and worker")
	s.PutFile(s.ControllerNode(0), "/tmp/k0s.yaml", k0sConfig)
	s.Require().NoError(s.InitController(0, "--config=/tmp/k0s.yaml"))
	s.Require().NoError(s.RunWorkers())

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

	s.T().Log("checking if etcd contains keys")
	etcdSSH, err := s.SSH(s.ExternalEtcd(0))
	s.Require().NoError(err)
	defer etcdSSH.Disconnect()

	output, err := etcdSSH.ExecWithOutput(
		"ETCDCTL_API=3 /opt/etcdctl --endpoints=http://127.0.0.1:2379 get /k0s-tenant/services/specs/kube-system/kube-dns --keys-only")
	s.Require().NoError(err)
	s.Require().Contains(output, "/k0s-tenant/services/specs/kube-system/kube-dns")

	etcdLeaveOutput, err := k0sControllerSSH.ExecWithOutput("/usr/local/bin/k0s etcd leave")
	s.Require().Error(err)
	s.Require().Contains(etcdLeaveOutput, "command 'k0s etcd' does not support external etcd cluster")

	etcdListOutput, err := k0sControllerSSH.ExecWithOutput("/usr/local/bin/k0s etcd member-list")
	s.Require().Error(err)
	s.Require().Contains(etcdListOutput, "command 'k0s etcd' does not support external etcd cluster")

	backupOutput, err := k0sControllerSSH.ExecWithOutput("/usr/local/bin/k0s backup")
	s.Require().Error(err)
	s.Require().Contains(backupOutput, "command 'k0s backup' does not support external etcd cluster")
}

func TestExternalEtcdSuite(t *testing.T) {
	s := ExternalEtcdSuite{
		common.FootlooseSuite{
			ControllerCount:  1,
			WorkerCount:      1,
			WithExternalEtcd: true,
		},
	}
	suite.Run(t, &s)
}
