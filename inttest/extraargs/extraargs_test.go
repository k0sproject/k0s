// SPDX-FileCopyrightText: 2021 k0s authors
// SPDX-License-Identifier: Apache-2.0

package extraargs

import (
	"fmt"
	"strconv"
	"testing"

	"github.com/k0sproject/k0s/inttest/common"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/stretchr/testify/suite"
)

type ExtraArgsSuite struct {
	common.BootlooseSuite
}

func (s *ExtraArgsSuite) TestK0sGetsUp() {
	ctx := s.Context()

	s.PutFile(s.ControllerNode(0), "/tmp/k0s.yaml", k0sConfig)
	s.NoError(s.InitController(0, "--config=/tmp/k0s.yaml"))

	s.NoError(s.RunWorkers())

	kc, err := s.KubeClient(s.ControllerNode(0))
	s.Require().NoError(err)

	err = s.WaitForNodeReady(s.WorkerNode(0), kc)
	s.NoError(err)

	s.AssertSomeKubeSystemPods(kc)

	sshCtrl, err := s.SSH(ctx, s.ControllerNode(0))
	defer sshCtrl.Disconnect()
	s.NoError(err)

	s.checkFlag(sshCtrl, "/var/lib/k0s/bin/kube-apiserver", "--disable-admission-plugins=PodSecurity")
	s.checkFlag(sshCtrl, "/var/lib/k0s/bin/kube-controller-manager", "--terminated-pod-gc-threshold=1000")
	s.checkFlag(sshCtrl, "/var/lib/k0s/bin/kube-controller-manager", "--concurrent-service-syncs=5")
	s.checkFlag(sshCtrl, "/var/lib/k0s/bin/kube-scheduler", "--v=4")
	s.checkFlag(sshCtrl, "/var/lib/k0s/bin/kube-scheduler", "--bind-address=0.0.0.0")
	s.checkFlag(sshCtrl, "/var/lib/k0s/bin/etcd", "--log-level=warn")
	s.checkFlagCount(sshCtrl, "/var/lib/k0s/bin/etcd", "--logger=zap", 3)

	sshWorker, err := s.SSH(ctx, s.WorkerNode(0))
	defer sshWorker.Disconnect()
	s.NoError(err)

	s.T().Log("waiting to see kube-proxy pods ready")
	s.NoError(common.WaitForDaemonSet(ctx, kc, "kube-proxy", metav1.NamespaceSystem), "kube-proxy did not start")
	s.checkFlag(sshWorker, "/usr/local/bin/kube-proxy", "--config-sync-period=12m0s")
	s.checkFlag(sshWorker, "/usr/local/bin/kube-proxy", "-v=2")

	s.T().Log("waiting to see kube-router pods ready")
	s.NoError(common.WaitForKubeRouterReady(ctx, kc), "kube-router did not start")
	s.checkFlag(sshWorker, "kube-router", "--enable-cni=true")
	s.checkFlag(sshWorker, "kube-router", "-v=0")
}

func (s *ExtraArgsSuite) checkFlagCount(ssh *common.SSHConnection, processName string, flag string, expectedCount int) {
	s.T().Logf("Checking flag %s in process %s", flag, processName)
	pid, err := ssh.ExecWithOutput(s.Context(), "/usr/bin/pgrep "+processName)
	s.NoError(err)

	flagCount, err := ssh.ExecWithOutput(s.Context(), fmt.Sprintf("/bin/grep -c -- %s /proc/%s/cmdline", flag, pid))
	// If there are no flags, grep returns 1, so we need to ignore this error. Checking the output should beenough.
	if expectedCount > 0 {
		s.NoError(err)
	}
	if flagCount != strconv.Itoa(expectedCount) {
		s.T().Fatalf("%s flag %s found %s, expected %d", processName, flag, flagCount, expectedCount)
	}

}

func (s *ExtraArgsSuite) checkFlag(ssh *common.SSHConnection, processName string, flag string) {
	s.checkFlagCount(ssh, processName, flag, 1)
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
    rawArgs:
    - --enable-admission-plugins=NamespaceLifecycle,LimitRanger,ServiceAccount,TaintNodesByCondition,Priority,DefaultTolerationSeconds,DefaultStorageClass,StorageObjectInUseProtection,PersistentVolumeClaimResize,RuntimeClass,CertificateApproval,CertificateSigning,CertificateSubjectRestriction,DefaultIngressClass,MutatingAdmissionWebhook,ValidatingAdmissionWebhook,ResourceQuota
  controllerManager:
    extraArgs:
      terminated-pod-gc-threshold: "1000"
    rawArgs:
    - --concurrent-service-syncs=5
  scheduler:
    extraArgs:
      v: "4"
    rawArgs:
    - --bind-address=0.0.0.0
  storage:
    etcd:
      extraArgs:
        log-level: warn
        logger: zap
      rawArgs:
      - --logger=zap
      - --logger=zap
  network:
    provider: kuberouter
    kubeRouter:
      extraArgs:
        enable-cni: "true"
      rawArgs:
      - -v=0
    kubeProxy:
      extraArgs:
        config-sync-period: 12m0s
      rawArgs:
      - -v=2
`
