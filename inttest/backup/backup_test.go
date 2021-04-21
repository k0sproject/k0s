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
	"context"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/k0sproject/k0s/inttest/common"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type BackupSuite struct {
	common.FootlooseSuite
}

func (s *BackupSuite) TestK0sGetsUp() {
	s.NoError(s.InitController(0))
	s.NoError(s.RunWorkers())

	kc, err := s.KubeClient(s.ControllerNode(0))
	s.NoError(err)
	s.NoError(s.WaitJoinAPI(s.ControllerNode(0)))
	token, err := s.GetJoinToken("controller")
	s.NoError(err)
	s.NoError(s.InitController(1, token))
	s.NoError(s.WaitJoinAPI(s.ControllerNode(1)))

	err = s.WaitForNodeReady(s.WorkerNode(0), kc)
	s.NoError(err)

	err = s.WaitForNodeReady(s.WorkerNode(1), kc)
	s.NoError(err)

	pods, err := kc.CoreV1().Pods("kube-system").List(context.TODO(), v1.ListOptions{
		Limit: 100,
	})
	s.NoError(err)

	podCount := len(pods.Items)

	s.T().Logf("found %d pods in kube-system", podCount)
	s.Greater(podCount, 0, "expecting to see few pods in kube-system namespace")

	s.T().Log("waiting to see kube-router pods ready")
	s.NoError(common.WaitForKubeRouterReady(kc), "kube-router did not start")

	// Take some UIDs to be able to verify state has restored properly
	ns, err := kc.CoreV1().Namespaces().Get(context.TODO(), "kube-system", v1.GetOptions{})
	s.NoError(err)

	s.NoError(s.takeBackup())

	s.NoError(s.StopController(s.ControllerNode(0)))
	_ = s.StopController(s.ControllerNode(1)) // No error check as k0s might have actually exited since etcd is not really happy

	s.NoError(s.Reset(s.ControllerNode(0)))
	s.NoError(s.Reset(s.ControllerNode(1)))

	s.NoError(s.restoreBackup())
	s.NoError(s.InitController(0))
	s.NoError(s.WaitJoinAPI(s.ControllerNode(0)))

	// Join the second controller as normally
	s.NoError(s.InitController(1, token))

	// Take some UIDs to be able to verify state has restored properly
	nsNew, err := kc.CoreV1().Namespaces().Get(context.TODO(), "kube-system", v1.GetOptions{})
	s.NoError(err)

	s.Equal(ns.ObjectMeta.UID, nsNew.ObjectMeta.UID)

	err = s.WaitForNodeReady(s.WorkerNode(0), kc)
	s.NoError(err)

	err = s.WaitForNodeReady(s.WorkerNode(1), kc)
	s.NoError(err)
}

func (s *BackupSuite) takeBackup() error {
	ssh, err := s.SSH(s.ControllerNode(0))
	if err != nil {
		return err
	}
	defer ssh.Disconnect()

	out, err := ssh.ExecWithOutput("k0s backup --save-path /root/")
	if err != nil {
		return err
	}
	s.T().Logf("backup taken succesfully with output:\n%s", out)
	return nil
}

func (s *BackupSuite) restoreBackup() error {
	ssh, err := s.SSH(s.ControllerNode(0))
	if err != nil {
		return err
	}
	defer ssh.Disconnect()

	out, err := ssh.ExecWithOutput("k0s restore $(ls /root/k0s_backup_*.tar.gz)")
	if err != nil {
		return err
	}
	s.T().Logf("restored succesfully with output:\n%s", out)

	return nil
}

func TestBackupSuite(t *testing.T) {
	s := BackupSuite{
		common.FootlooseSuite{
			ControllerCount: 2,
			WorkerCount:     2,
		},
	}
	suite.Run(t, &s)
}
