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
	"bytes"
	"context"
	"html/template"
	"testing"

	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"

	"github.com/stretchr/testify/suite"

	"github.com/k0sproject/k0s/inttest/common"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const configWithExternaladdress = `
apiVersion: k0s.k0sproject.io/v1beta1
kind: ClusterConfig
metadata:
  name: k0s
spec:
  api:
    externalAddress: {{ .Address }}
`

type BackupSuite struct {
	common.FootlooseSuite
}

func (s *BackupSuite) getControllerConfig(ipAddress string) string {
	data := struct {
		Address string
	}{
		Address: ipAddress,
	}
	content := bytes.NewBuffer([]byte{})
	s.Require().NoError(template.Must(template.New("k0s.yaml").Parse(configWithExternaladdress)).Execute(content, data), "can't execute k0s.yaml template")
	return content.String()
}

func (s *BackupSuite) TestK0sGetsUp() {
	ipAddress := s.GetControllerIPAddress(0)
	s.T().Logf("ip address: %s", ipAddress)
	config := s.getControllerConfig(ipAddress)
	s.PutFile("controller0", "/tmp/k0s.yaml", config)
	s.PutFile("controller1", "/tmp/k0s.yaml", config)

	s.Require().NoError(s.InitController(0, "--config=/tmp/k0s.yaml"))
	s.Require().NoError(s.RunWorkers())

	kc, err := s.KubeClient(s.ControllerNode(0))
	s.Require().NoError(err)
	s.Require().NoError(s.WaitJoinAPI(s.ControllerNode(0)))
	token, err := s.GetJoinToken("controller")
	s.Require().NoError(err)
	s.Require().NoError(s.InitController(1, token, "--config=/tmp/k0s.yaml"))
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
	// Matching object UIDs after restore guarantees we got the full state restored
	s.Require().Equal(snapshot, snapshotAfterBackup)
}

type snapshot struct {
	namespaces map[types.UID]string
	services   map[types.UID]string
	nodes      map[types.UID]string
}

func (s *BackupSuite) makeSnapshot(kc *kubernetes.Clientset) snapshot {
	// Take some UIDs to be able to verify state has restored properly
	namespaces := make(map[types.UID]string)
	nsList, err := kc.CoreV1().Namespaces().List(context.TODO(), v1.ListOptions{})
	s.Require().NoError(err)
	for _, n := range nsList.Items {
		namespaces[n.ObjectMeta.UID] = n.Name
	}

	services := make(map[types.UID]string)
	{
		svc, err := kc.CoreV1().Services("default").Get(context.TODO(), "kubernetes", v1.GetOptions{})
		s.Require().NoError(err)
		services[svc.ObjectMeta.UID] = svc.Name
	}

	nodes := make(map[types.UID]string)
	nodeList, err := kc.CoreV1().Nodes().List(context.TODO(), v1.ListOptions{})
	s.Require().NoError(err)
	for _, n := range nodeList.Items {
		nodes[n.ObjectMeta.UID] = n.Name
	}

	return snapshot{
		namespaces: namespaces,
		services:   services,
		nodes:      nodes,
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
