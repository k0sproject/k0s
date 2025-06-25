// SPDX-FileCopyrightText: 2021 k0s authors
// SPDX-License-Identifier: Apache-2.0

package extraargs

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/k0sproject/k0s/inttest/common"
)

type ExtraArgsSuite struct {
	common.BootlooseSuite
}

func (s *ExtraArgsSuite) TestK0sGetsUp() {
	s.PutFile(s.ControllerNode(0), "/tmp/k0s.yaml", k0sConfig)
	s.NoError(s.InitController(0, "--config=/tmp/k0s.yaml"))

	s.NoError(s.RunWorkers())

	kc, err := s.KubeClient(s.ControllerNode(0))
	s.Require().NoError(err)

	err = s.WaitForNodeReady(s.WorkerNode(0), kc)
	s.NoError(err)

	s.AssertSomeKubeSystemPods(kc)

	s.T().Log("waiting to see kube-router pods ready")
	s.NoError(common.WaitForKubeRouterReady(s.Context(), kc), "kube-router did not start")

	sshCtrl, err := s.SSH(s.Context(), s.ControllerNode(0))
	defer sshCtrl.Disconnect()
	s.NoError(err)

	s.checkFlag(sshCtrl, "/var/lib/k0s/bin/kube-apiserver", "--disable-admission-plugins=PodSecurity")
	s.checkFlag(sshCtrl, "/var/lib/k0s/bin/etcd", "--logger=zap")

	sshWorker, err := s.SSH(s.Context(), s.WorkerNode(0))
	defer sshWorker.Disconnect()
	s.NoError(err)

	s.checkFlag(sshWorker, "/usr/local/bin/kube-proxy", "--config-sync-period=12m0s")

}
func (s *ExtraArgsSuite) checkFlag(ssh *common.SSHConnection, processName string, flag string) {
	s.T().Logf("Checking flag %s in process %s", flag, processName)
	pid, err := ssh.ExecWithOutput(s.Context(), "/usr/bin/pgrep "+processName)
	s.NoError(err)

	flagCount, err := ssh.ExecWithOutput(s.Context(), fmt.Sprintf("/bin/grep -c -- %s /proc/%s/cmdline", flag, pid))
	s.NoError(err)
	if flagCount != "1" {
		s.T().Fatalf("%s flag %s not found", processName, flag)
	}

}

func TestExtraArgsSuite(t *testing.T) {
	s :=
		ExtraArgsSuite{
			common.BootlooseSuite{
				ControllerCount: 1,
				WorkerCount:     1,
			},
		}
	suite.Run(t, &s)
}

const k0sConfig = `
spec:
  api:
    extraArgs:
      disable-admission-plugins: PodSecurity
      enable-admission-plugins: NamespaceLifecycle,LimitRanger,ServiceAccount,TaintNodesByCondition,Priority,DefaultTolerationSeconds,DefaultStorageClass,StorageObjectInUseProtection,PersistentVolumeClaimResize,RuntimeClass,CertificateApproval,CertificateSigning,CertificateSubjectRestriction,DefaultIngressClass,MutatingAdmissionWebhook,ValidatingAdmissionWebhook,ResourceQuota
  storage:
    etcd:
      extraArgs:
        logger: zap
  network:
    kubeProxy:
      extraArgs:
        config-sync-period: 12m0s
`
