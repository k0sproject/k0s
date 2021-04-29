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
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"sort"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/k0sproject/k0s/inttest/common"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type BackupSuite struct {
	common.FootlooseSuite
}

func (s *BackupSuite) TestK0sGetsUp() {
	s.Require().NoError(s.InitController(0))
	s.Require().NoError(s.RunWorkers())

	kc, err := s.KubeClient(s.ControllerNode(0))
	s.Require().NoError(err)
	s.Require().NoError(s.WaitJoinAPI(s.ControllerNode(0)))
	token, err := s.GetJoinToken("controller")
	s.Require().NoError(err)
	s.Require().NoError(s.InitController(1, token))
	s.Require().NoError(s.WaitJoinAPI(s.ControllerNode(1)))

	err = s.WaitForNodeReady(s.WorkerNode(0), kc)
	s.Require().NoError(err)

	err = s.WaitForNodeReady(s.WorkerNode(1), kc)
	s.Require().NoError(err)

	pods, err := kc.CoreV1().Pods("kube-system").List(context.TODO(), v1.ListOptions{
		Limit: 100,
	})
	s.Require().NoError(err)

	podCount := len(pods.Items)

	s.T().Logf("found %d pods in kube-system", podCount)
	s.Greater(podCount, 0, "expecting to see few pods in kube-system namespace")

	s.T().Log("waiting to see kube-router pods ready")
	s.Require().NoError(common.WaitForKubeRouterReady(kc), "kube-router did not start")

	s.Require().NoError(s.takeBackup())

	snapshot := s.makeSnapshot(kc)

	s.Require().NoError(err)

	s.Require().NoError(s.StopController(s.ControllerNode(0)))
	_ = s.StopController(s.ControllerNode(1)) // No error check as k0s might have actually exited since etcd is not really happy

	s.Require().NoError(s.Reset(s.ControllerNode(0)))
	s.Require().NoError(s.Reset(s.ControllerNode(1)))

	s.Require().NoError(s.restoreBackup())
	s.Require().NoError(s.InitController(0))
	s.Require().NoError(s.WaitJoinAPI(s.ControllerNode(0)))

	// Join the second controller as normally
	s.Require().NoError(s.InitController(1, token))

	s.Require().NoError(err)

	err = s.WaitForNodeReady(s.WorkerNode(0), kc)
	s.Require().NoError(err)

	err = s.WaitForNodeReady(s.WorkerNode(1), kc)
	s.Require().NoError(err)

	snapshotAfterBackup := s.makeSnapshot(kc)
	s.Require().NoError(err)
	s.Require().True(snapshot.hasServices(2))
	s.Require().True(snapshot.hasDeployments(2))
	s.Require().True(snapshot.hasDaemonSets(3))
	s.Require().Equal(snapshot, snapshotAfterBackup)
}

type snapshot struct {
	systemNs    types.UID
	daemonSets  []types.UID
	deployments []types.UID
	services    []types.UID
}

func (s snapshot) hasDaemonSets(n int) bool {
	return len(s.daemonSets) == n
}

func (s snapshot) hasServices(n int) bool {
	return len(s.services) == n
}

func (s snapshot) hasDeployments(n int) bool {
	return len(s.deployments) == n
}

func (s *BackupSuite) makeSnapshot(kc *kubernetes.Clientset) snapshot {
	// Take some UIDs to be able to verify state has restored properly
	nsNew, err := kc.CoreV1().Namespaces().Get(context.TODO(), "kube-system", v1.GetOptions{})
	s.Require().NoError(err)

	daemonSets := []types.UID{}
	deployments := []types.UID{}
	services := []types.UID{}
	{
		response, err := kc.AppsV1().DaemonSets("kube-system").List(context.TODO(), v1.ListOptions{})
		s.Require().NoError(err)
		for _, obj := range response.Items {
			daemonSets = append(daemonSets, obj.ObjectMeta.UID)
		}
	}
	{
		response, err := kc.AppsV1().Deployments("kube-system").List(context.TODO(), v1.ListOptions{})
		s.Require().NoError(err)

		for _, obj := range response.Items {
			deployments = append(deployments, obj.ObjectMeta.UID)
		}
	}
	{
		response, err := kc.CoreV1().Services("kube-system").List(context.TODO(), v1.ListOptions{})
		s.Require().NoError(err)

		for _, obj := range response.Items {
			services = append(services, obj.ObjectMeta.UID)
		}
	}

	sort.Slice(daemonSets, func(i, j int) bool {
		return daemonSets[i] > daemonSets[j]
	})
	sort.Slice(deployments, func(i, j int) bool {
		return deployments[i] > deployments[j]
	})
	sort.Slice(services, func(i, j int) bool {
		return services[i] > services[j]
	})

	return snapshot{
		systemNs:    nsNew.ObjectMeta.UID,
		daemonSets:  daemonSets,
		deployments: deployments,
		services:    services,
	}
}

func (s *BackupSuite) takeBackup() error {
	ssh, err := s.SSH(s.ControllerNode(0))
	if err != nil {
		return err
	}
	defer ssh.Disconnect()

	out, err := ssh.ExecWithOutput("k0s backup --save-path /root/")
	if err != nil {
		s.T().Errorf("backup failed with output:\n%s", out)
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
		s.T().Errorf("restored failed with output:\n%s", out)
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
