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

package basic

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/k0sproject/k0s/inttest/common"
	"github.com/k0sproject/k0s/pkg/apis/k0s/v1beta1"
	"github.com/k0sproject/k0s/pkg/constant"
	"github.com/k0sproject/k0s/pkg/kubernetes/watch"

	certificatesv1 "k8s.io/api/certificates/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/kubernetes"

	"github.com/BurntSushi/toml"
	"github.com/stretchr/testify/suite"
)

type BasicSuite struct {
	common.BootlooseSuite
}

type (
	CSR     = certificatesv1.CertificateSigningRequest
	CSRList = certificatesv1.CertificateSigningRequestList
)

func (s *BasicSuite) TestK0sGetsUp() {
	ctx := s.Context()
	customDataDir := "/var/lib/k0s/custom-data-dir"

	// Create an empty file to prove that k0s manage to rewrite a partially written file
	ssh, err := s.SSH(ctx, s.ControllerNode(0))
	s.Require().NoError(err)
	defer ssh.Disconnect()
	_, err = ssh.ExecWithOutput(ctx, fmt.Sprintf("mkdir -p %s/bin && touch -t 202201010000 %s/bin/kube-apiserver", customDataDir, customDataDir))
	s.Require().NoError(err)
	_, err = ssh.ExecWithOutput(ctx, fmt.Sprintf("touch -t 202201010000 %s", s.K0sFullPath))
	s.Require().NoError(err)
	_, err = ssh.ExecWithOutput(ctx, "mkdir -p /run/k0s/konnectivity-server/ && touch -t 202201010000 /run/k0s/konnectivity-server/konnectivity-server.sock")
	s.Require().NoError(err)

	dataDirOpt := fmt.Sprintf("--data-dir=%s", customDataDir)
	s.Require().NoError(s.InitController(0, dataDirOpt))

	token, err := s.GetJoinToken("worker", dataDirOpt)
	s.Require().NoError(err)
	s.NoError(s.RunWorkersWithToken(token, `--labels="k0sproject.io/foo=bar"`, `--kubelet-extra-args=" --address=0.0.0.0  --event-burst=10"`))

	kc, err := s.KubeClient(s.ControllerNode(0), dataDirOpt)
	if err != nil {
		s.FailNow("failed to obtain Kubernetes client", err)
	}
	restConfig, err := s.GetKubeConfig(s.ControllerNode(0), "")
	s.Require().NoError(err)

	err = s.WaitForNodeReady(s.WorkerNode(0), kc)
	s.NoError(err)

	if labels, err := s.GetNodeLabels(s.WorkerNode(0), kc); s.NoError(err) {
		s.Equal("bar", labels["k0sproject.io/foo"])
	}

	err = s.WaitForNodeReady(s.WorkerNode(1), kc)
	s.NoError(err)

	s.AssertSomeKubeSystemPods(kc)

	s.T().Log("waiting to see kube-router pods ready")
	s.NoError(common.WaitForKubeRouterReady(ctx, kc), "kube-router did not start")

	s.Require().NoError(s.checkCertPerms(ctx, s.ControllerNode(0)))

	s.T().Log("Waiting for all worker CSRs to be approved")
	s.Require().NoError(s.checkCSRs(ctx, kc))

	s.Require().NoError(s.verifyKubeletAddressFlag(ctx, s.WorkerNode(0)))
	s.Require().NoError(s.verifyKubeletAddressFlag(ctx, s.WorkerNode(1)))
	for _, lease := range []string{"kube-scheduler", "kube-controller-manager"} {
		s.T().Logf("Waiting for %s lease", lease)
		_, err := common.WaitForLease(ctx, kc, lease, "kube-system")
		s.Require().NoError(err, lease)
	}

	// We need to first wait till we see pod logs, that's a signal that konnectivity tunnels are up and thus we can then connect to kubelet
	// via the API.
	s.Require().NoError(common.WaitForPodLogs(ctx, kc, "kube-system"))
	for i := 0; i < s.WorkerCount; i++ {
		node := s.WorkerNode(i)
		s.T().Logf("checking that we can connect to kubelet metrics on %s", node)
		s.Require().NoError(common.VerifyKubeletMetrics(ctx, kc, node))
	}

	s.T().Log("checking kube-router gobgp functionality")
	kubeRouterPods, err := kc.CoreV1().Pods("kube-system").List(ctx, metav1.ListOptions{LabelSelector: "k8s-app=kube-router"})
	s.Require().NoError(err)
	// Just take the first running pod for execing the gobgp command
	for _, pod := range kubeRouterPods.Items {
		if pod.Status.Phase == corev1.PodRunning {
			out, err := common.PodExecCmdOutput(kc, restConfig, pod.Name, "kube-system", "gobgp global")
			s.Require().NoError(err)
			// Check that the output contains the default AS number, that's a sign that gobgp is working
			s.Assert().Regexp(`AS:\s+64512`, out)
			break
		}
	}

	s.verifyContainerdDefaultConfig(ctx)

	s.Require().NoError(s.probeCoreDNSAntiAffinity(ctx, kc))
}

func (s *BasicSuite) checkCertPerms(ctx context.Context, node string) error {
	ssh, err := s.SSH(ctx, node)
	if err != nil {
		return err
	}
	defer ssh.Disconnect()

	output, err := ssh.ExecWithOutput(ctx, `find /var/lib/k0s/custom-data-dir/pki/  \( -name '*.key' -o -name '*.conf' \) -a \! -perm 0640`)
	if err != nil {
		return err
	}

	if output != "" {
		return fmt.Errorf("some private files having non 640 permissions: %s", output)
	}

	return nil
}

// Verifies that kubelet process has the address flag set
func (s *BasicSuite) verifyKubeletAddressFlag(ctx context.Context, node string) error {
	ssh, err := s.SSH(ctx, node)
	if err != nil {
		return err
	}
	defer ssh.Disconnect()

	output, err := ssh.ExecWithOutput(ctx, `grep -e '--address=0.0.0.0' /proc/$(pidof kubelet)/cmdline`)
	if err != nil {
		return err
	}
	if output != "--address=0.0.0.0" {
		return fmt.Errorf("kubelet does not have the address flag set")
	}

	return nil
}

func (s *BasicSuite) checkCSRs(ctx context.Context, kc *kubernetes.Clientset) error {
	// Wait until CSRs for all worker nodes got signed
	approvedNodes := map[string]struct{}{}

	return watch.FromClient[*CSRList, CSR](kc.CertificatesV1().CertificateSigningRequests()).
		WithFieldSelector(fields.OneTermEqualSelector("spec.signerName", "kubernetes.io/kubelet-serving")).
		WithErrorCallback(common.RetryWatchErrors(s.T().Logf)).
		Until(ctx, func(csr *CSR) (bool, error) {
			if !strings.HasPrefix(csr.Spec.Username, "system:node:worker") {
				return false, nil
			}
			if _, alreadyApproved := approvedNodes[csr.Spec.Username]; alreadyApproved {
				return false, nil
			}

			if reason, ok := getCSRApprovalReason(csr); !ok {
				s.T().Logf("CSR for %s is not yet approved", csr.Spec.Username)
				return false, nil
			} else if reason != "Autoapproved by K0s CSRApprover" {
				return false, fmt.Errorf("CSR for %s has an unexpected approval reason: %q", csr.Spec.Username, reason)
			}

			s.T().Logf("CSR for %s is approved", csr.Spec.Username)

			approvedNodes[csr.Spec.Username] = struct{}{}
			if len(approvedNodes) == s.WorkerCount {
				return true, nil
			}

			return false, nil
		})
}

func getCSRApprovalReason(csr *CSR) (string, bool) {
	for _, condition := range csr.Status.Conditions {
		if condition.Type != certificatesv1.CertificateApproved {
			continue
		}
		return condition.Reason, true
	}

	return "", false
}

func (s *BasicSuite) verifyContainerdDefaultConfig(ctx context.Context) {
	var defaultConfig bytes.Buffer
	ssh, err := s.SSH(ctx, s.WorkerNode(0))
	if !s.NoError(err) {
		return
	}
	defer ssh.Disconnect()

	if !s.NoError(ssh.Exec(ctx, "/var/lib/k0s/bin/containerd --config=/etc/k0s/containerd.toml config dump", common.SSHStreams{Out: &defaultConfig})) {
		return
	}

	var parsedConfig struct {
		Plugins struct {
			CRI struct {
				SandboxImage string `toml:"sandbox_image"`
			} `toml:"io.containerd.grpc.v1.cri"`
		} `toml:"plugins"`
	}

	_, err = toml.Decode(defaultConfig.String(), &parsedConfig)
	if !s.NoError(err) {
		return
	}

	s.Equal((&v1beta1.ImageSpec{
		Image:   constant.KubePauseContainerImage,
		Version: constant.KubePauseContainerImageVersion,
	}).URI(), parsedConfig.Plugins.CRI.SandboxImage)
}

func (s *BasicSuite) probeCoreDNSAntiAffinity(ctx context.Context, kc *kubernetes.Clientset) error {
	// Wait until both CoreDNS Pods got assigned to a node
	pods := map[string]string{}

	return watch.Pods(kc.CoreV1().Pods("kube-system")).
		WithLabels(labels.Set{"k8s-app": "kube-dns"}).
		WithErrorCallback(common.RetryWatchErrors(s.T().Logf)).
		Until(ctx, func(pod *corev1.Pod) (bool, error) {
			// Keep waiting until there's anti-affinity and node assignment
			if a := pod.Spec.Affinity; a == nil || a.PodAntiAffinity == nil {
				s.T().Logf("Pod %s doesn't have any pod anti-affinity", pod.Name)
				return false, nil
			}
			nodeName := pod.Spec.NodeName
			if nodeName == "" {
				s.T().Logf("Pod %s not scheduled yet: %+v", pod.ObjectMeta.Name, pod.Status)
				return false, nil
			}

			if prevName, ok := pods[nodeName]; ok && pod.Name != prevName {
				return false, fmt.Errorf("multiple CoreDNS pods scheduled on node %s: %s and %s", nodeName, prevName, pod.Name)
			}

			s.T().Logf("Pod %s scheduled on %s", pod.Name, pod.Spec.NodeName)

			pods[nodeName] = pod.Name
			return len(pods) > 1, nil
		})
}

func TestBasicSuite(t *testing.T) {
	s := BasicSuite{
		common.BootlooseSuite{
			ControllerCount: 1,
			WorkerCount:     2,
		},
	}
	suite.Run(t, &s)
}
