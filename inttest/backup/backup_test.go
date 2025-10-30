// SPDX-FileCopyrightText: 2021 k0s authors
// SPDX-License-Identifier: Apache-2.0

package basic

import (
	"testing"
	"time"

	"github.com/k0sproject/k0s/inttest/common"
	"github.com/k0sproject/k0s/pkg/apis/k0s/v1beta1"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"

	"sigs.k8s.io/yaml"

	"github.com/stretchr/testify/suite"
)

type BackupSuite struct {
	common.BootlooseSuite
	backupFunc  func() error
	restoreFunc func() error
}

func (s *BackupSuite) getControllerConfig(ipAddress string) string {
	config := v1beta1.ClusterConfig{
		Spec: &v1beta1.ClusterSpec{
			API: &v1beta1.APISpec{
				ExternalAddress: ipAddress,
			},
		},
	}
	yaml, err := yaml.Marshal(&config)
	s.Require().NoError(err)
	return string(yaml)
}

func (s *BackupSuite) TestK0sGetsUp() {
	ipAddress := s.GetControllerIPAddress(0)
	s.T().Logf("ip address: %s", ipAddress)
	config := s.getControllerConfig(ipAddress)
	s.T().Log("Config:", config)
	s.PutFile("controller0", "/tmp/k0s.yaml", config)
	s.PutFile("controller1", "/tmp/k0s.yaml", config)

	s.Require().NoError(s.InitController(0, "--config=/tmp/k0s.yaml", "--enable-worker"))
	s.Require().NoError(s.RunWorkers())

	kc, err := s.KubeClient(s.ControllerNode(0))
	s.Require().NoError(err)
	s.Require().NoError(s.WaitJoinAPI(s.ControllerNode(0)))
	token, err := s.GetJoinToken("controller")
	s.Require().NoError(err)
	s.Require().NoError(s.InitController(1, token, "--config=/tmp/k0s.yaml"))
	s.Require().NoError(s.WaitJoinAPI(s.ControllerNode(1)))

	s.Require().NoError(s.WaitForNodeReady(s.WorkerNode(0), kc))
	s.Require().NoError(s.WaitForNodeReady(s.WorkerNode(1), kc))

	s.AssertSomeKubeSystemPods(kc)

	s.T().Log("waiting to see kube-router pods ready")
	s.Require().NoError(common.WaitForKubeRouterReady(s.Context(), kc), "kube-router did not start")

	s.Require().NoError(s.backupFunc())

	snapshot := s.makeSnapshot(kc)

	s.Require().NoError(s.StopController(s.ControllerNode(0)))
	_ = s.StopController(s.ControllerNode(1)) // No error check as k0s might have actually exited since etcd is not really happy

	s.reset(s.ControllerNode(0))
	s.reset(s.ControllerNode(1))

	s.Require().NoError(s.restoreFunc())
	s.Require().NoError(s.InitController(0, "--enable-worker"))
	s.Require().NoError(s.WaitJoinAPI(s.ControllerNode(0)))

	// Join the second controller as normally
	s.Require().NoError(s.InitController(1, "--enable-worker", token))

	s.Require().NoError(s.WaitForNodeReady(s.WorkerNode(0), kc))
	s.Require().NoError(s.WaitForNodeReady(s.WorkerNode(1), kc))

	snapshotAfterBackup := s.makeSnapshot(kc)
	// Matching object UIDs after restore guarantees we got the full state restored
	s.Require().Equal(snapshot, snapshotAfterBackup)

	s.Require().NoError(s.VerifyFileSystemRestore())
}

func (s *BackupSuite) reset(name string) {
	ssh, err := s.SSH(s.Context(), name)
	s.Require().NoError(err)
	defer ssh.Disconnect()
	s.Require().NoError(ssh.Exec(s.Context(), "k0s reset --debug", common.SSHStreams{}))
}

type snapshot struct {
	namespaces map[types.UID]string
	services   map[types.UID]string
	nodes      map[types.UID]string
}

func (s *BackupSuite) makeSnapshot(kc *kubernetes.Clientset) snapshot {
	// Take some UIDs to be able to verify state has restored properly
	namespaces := make(map[types.UID]string)
	nsList, err := kc.CoreV1().Namespaces().List(s.Context(), metav1.ListOptions{})
	s.Require().NoError(err)
	for _, n := range nsList.Items {
		namespaces[n.UID] = n.Name
	}

	services := make(map[types.UID]string)
	{
		svc, err := kc.CoreV1().Services("default").Get(s.Context(), "kubernetes", metav1.GetOptions{})
		s.Require().NoError(err)
		services[svc.UID] = svc.Name
	}

	nodes := make(map[types.UID]string)
	nodeList, err := kc.CoreV1().Nodes().List(s.Context(), metav1.ListOptions{})
	s.Require().NoError(err)
	for _, n := range nodeList.Items {
		nodes[n.UID] = n.Name
	}

	return snapshot{
		namespaces: namespaces,
		services:   services,
		nodes:      nodes,
	}
}

func (s *BackupSuite) VerifyFileSystemRestore() error {
	ssh, err := s.SSH(s.Context(), s.ControllerNode(0))
	if err != nil {
		return err
	}
	defer ssh.Disconnect()

	// Checking for containerd should be enough given https://github.com/k0sproject/k0s/issues/2420
	// containerd may take a bit to start so we want to retry a few times
	checkPID := func() bool {
		_, err = ssh.ExecWithOutput(s.Context(), "/bin/pidof /var/lib/k0s/bin/containerd")
		return err == nil
	}

	s.Eventuallyf(checkPID, 180*time.Second, 10*time.Second,
		"fetching pidof containerd failed after 3 minutes with error: %v", err)
	return nil
}

func (s *BackupSuite) takeBackup() error {
	ssh, err := s.SSH(s.Context(), s.ControllerNode(0))
	if err != nil {
		return err
	}
	defer ssh.Disconnect()

	out, err := ssh.ExecWithOutput(s.Context(), "/usr/local/bin/k0s backup --debug --save-path /root/")
	if !s.NoErrorf(err, "backup failed with output: %s", out) {
		return err
	}
	s.T().Logf("backup taken successfully with output:\n%s", out)
	return nil
}

func (s *BackupSuite) takeBackupStdout() error {
	ssh, err := s.SSH(s.Context(), s.ControllerNode(0))
	if err != nil {
		return err
	}
	defer ssh.Disconnect()

	out, err := ssh.ExecWithOutput(s.Context(), "/usr/local/bin/k0s backup --debug --save-path - > backup.tar.gz")
	if !s.NoErrorf(err, "backup failed with output: %s", out) {
		return err
	}

	out, err = ssh.ExecWithOutput(s.Context(), "tar tf backup.tar.gz")
	if !s.NoErrorf(err, "backup inspection failed with output: %s", out) {
		return err
	}

	s.T().Logf("backup taken successfully with output:\n%s", out)
	return nil
}

func (s *BackupSuite) restoreBackup() error {
	ssh, err := s.SSH(s.Context(), s.ControllerNode(0))
	if err != nil {
		return err
	}
	defer ssh.Disconnect()

	s.T().Log("restoring controller from file")

	out, err := ssh.ExecWithOutput(s.Context(), "/usr/local/bin/k0s restore --debug $(ls /root/k0s_backup_*.tar.gz)")
	if !s.NoErrorf(err, "restore failed with output: %s", out) {
		return err
	}
	s.T().Logf("restored successfully with output:\n%s", out)

	return nil
}

func (s *BackupSuite) restoreBackupStdin() error {
	ssh, err := s.SSH(s.Context(), s.ControllerNode(0))
	if err != nil {
		return err
	}
	defer ssh.Disconnect()

	s.T().Log("restoring controller from stdin")

	out, err := ssh.ExecWithOutput(s.Context(), "cat backup.tar.gz | /usr/local/bin/k0s restore --debug -")
	if !s.NoErrorf(err, "restore failed with output: %s", out) {
		return err
	}
	s.T().Logf("restored successfully with output:\n%s", out)

	return nil
}

func TestBackupSuite(t *testing.T) {
	s := BackupSuite{
		BootlooseSuite: common.BootlooseSuite{
			ControllerCount: 2,
			WorkerCount:     2,
		},
	}
	s.backupFunc = s.takeBackup
	s.restoreFunc = s.restoreBackup
	suite.Run(t, &s)
}

func TestBackupSuiteStream(t *testing.T) {
	s := BackupSuite{
		BootlooseSuite: common.BootlooseSuite{
			ControllerCount: 2,
			WorkerCount:     2,
		},
	}
	s.backupFunc = s.takeBackupStdout
	s.restoreFunc = s.restoreBackupStdin
	suite.Run(t, &s)
}
