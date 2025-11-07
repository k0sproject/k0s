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
	useKine     bool
}

func (s *BackupSuite) getControllerConfig() string {
	config := v1beta1.ClusterConfig{
		Spec: &v1beta1.ClusterSpec{
			Network: &v1beta1.Network{
				NodeLocalLoadBalancing: &v1beta1.NodeLocalLoadBalancing{
					Enabled: s.ControllerCount > 1,
				},
			},
		},
	}

	if s.useKine {
		config.Spec.Storage = &v1beta1.StorageSpec{
			Type: v1beta1.KineStorageType,
		}
	}

	yaml, err := yaml.Marshal(&config)
	s.Require().NoError(err)
	return string(yaml)
}

func (s *BackupSuite) TestK0sGetsUp() {
	config := s.getControllerConfig()
	s.T().Log("Config:", config)
	s.PutFile(s.ControllerNode(0), "/tmp/k0s.yaml", config)

	s.Require().NoError(s.InitController(0, "--config=/tmp/k0s.yaml", "--enable-worker"))
	s.Require().NoError(s.RunWorkers())

	kc, err := s.KubeClient(s.ControllerNode(0))
	s.Require().NoError(err)

	var token string
	if s.ControllerCount > 1 {
		s.Require().NoError(s.WaitJoinAPI(s.ControllerNode(0)))
		token, err = s.GetJoinToken("controller")
		s.Require().NoError(err)

		for i := range s.ControllerCount - 1 {
			i := i + 1
			s.PutFile(s.ControllerNode(i), "/tmp/k0s.yaml", config)
			s.Require().NoError(s.InitController(i, token, "--config=/tmp/k0s.yaml"))
		}
	}

	for i := range s.WorkerCount {
		s.Require().NoError(s.WaitForNodeReady(s.WorkerNode(i), kc))
	}

	s.AssertSomeKubeSystemPods(kc)

	s.T().Log("waiting to see kube-router pods ready")
	s.Require().NoError(common.WaitForKubeRouterReady(s.Context(), kc), "kube-router did not start")

	s.Require().NoError(s.backupFunc())

	snapshot := s.makeSnapshot(kc)

	for i := range s.ControllerCount {
		s.Require().NoError(s.StopController(s.ControllerNode(i)))
		s.reset(s.ControllerNode(i))
	}

	s.Require().NoError(s.restoreFunc())
	s.Require().NoError(s.InitController(0, "--enable-worker"))

	// Join the other controllers in the usual way
	if s.ControllerCount > 1 {
		s.Require().NoError(s.WaitJoinAPI(s.ControllerNode(0)))
		for i := range s.ControllerCount - 1 {
			i := i + 1
			s.PutFile(s.ControllerNode(i), "/etc/k0s/k0s.yaml", config)
			s.Require().NoError(s.InitController(i, "--enable-worker", token))
		}
	}

	for i := range s.WorkerCount - 1 {
		s.Require().NoError(s.WaitForNodeReady(s.WorkerNode(i), kc))
	}

	snapshotAfterBackup := s.makeSnapshot(kc)
	// Matching object UIDs after restore guarantees we got the full state restored
	s.Require().Equal(snapshot, snapshotAfterBackup)

	s.Require().NoError(s.VerifyFileSystemRestore())
}

func (s *BackupSuite) reset(name string) {
	ssh, err := s.SSH(s.Context(), name)
	s.Require().NoError(err)
	defer ssh.Disconnect()
	s.Require().NoError(ssh.Exec(s.Context(), `
		set -eu
		k0s reset --debug
		rm /tmp/k0s.yaml
	`, common.SSHStreams{}))
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
		svc, err := kc.CoreV1().Services(metav1.NamespaceDefault).Get(s.Context(), "kubernetes", metav1.GetOptions{})
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

	out, err := ssh.ExecWithOutput(s.Context(), "/usr/local/bin/k0s restore --debug --config-out /etc/k0s/k0s.yaml $(ls /root/k0s_backup_*.tar.gz)")
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
			ControllerCount: 3,
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
			ControllerCount: 1,
			WorkerCount:     1,
		},
	}
	s.backupFunc = s.takeBackupStdout
	s.restoreFunc = s.restoreBackupStdin
	suite.Run(t, &s)
}

func TestBackupSuiteKine(t *testing.T) {
	s := BackupSuite{
		BootlooseSuite: common.BootlooseSuite{
			ControllerCount: 1,
			WorkerCount:     2,
		},
		useKine: true,
	}
	s.backupFunc = s.takeBackup
	s.restoreFunc = s.restoreBackup
	suite.Run(t, &s)
}
