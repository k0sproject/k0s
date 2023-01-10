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

package basic

import (
	"bytes"
	"fmt"
	"testing"
	"time"

	"github.com/k0sproject/k0s/inttest/common"
	"github.com/k0sproject/k0s/pkg/apis/k0s.k0sproject.io/v1beta1"
	"github.com/k0sproject/k0s/pkg/constant"

	certificatesv1 "k8s.io/api/certificates/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"

	"github.com/BurntSushi/toml"
	"github.com/stretchr/testify/suite"
)

type BasicSuite struct {
	common.FootlooseSuite
}

func (s *BasicSuite) TestK0sGetsUp() {
	customDataDir := "/var/lib/k0s/custom-data-dir"

	// Create an empty file to prove that k0s manage to rewrite a partially written file
	ssh, err := s.SSH(s.ControllerNode(0))
	s.Require().NoError(err)
	defer ssh.Disconnect()
	_, err = ssh.ExecWithOutput(s.Context(), fmt.Sprintf("mkdir -p %s/bin && touch -t 202201010000 %s/bin/kube-apiserver", customDataDir, customDataDir))
	s.Require().NoError(err)
	_, err = ssh.ExecWithOutput(s.Context(), fmt.Sprintf("touch -t 202201010000 %s", s.K0sFullPath))
	s.Require().NoError(err)
	_, err = ssh.ExecWithOutput(s.Context(), "mkdir -p /run/k0s/konnectivity-server/ && touch -t 202201010000 /run/k0s/konnectivity-server/konnectivity-server.sock")
	s.Require().NoError(err)

	dataDirOpt := fmt.Sprintf("--data-dir=%s", customDataDir)
	s.NoError(s.InitController(0, dataDirOpt))

	token, err := s.GetJoinToken("worker", dataDirOpt)
	s.Require().NoError(err)
	s.NoError(s.RunWorkersWithToken(token, `--labels="k0sproject.io/foo=bar"`, `--kubelet-extra-args=" --address=0.0.0.0  --event-burst=10"`))

	kc, err := s.KubeClient(s.ControllerNode(0), dataDirOpt)
	if err != nil {
		s.FailNow("failed to obtain Kubernetes client", err)
	}

	err = s.WaitForNodeReady(s.WorkerNode(0), kc)
	s.NoError(err)

	if labels, err := s.GetNodeLabels(s.WorkerNode(0), kc); s.NoError(err) {
		s.Equal("bar", labels["k0sproject.io/foo"])
	}

	err = s.WaitForNodeReady(s.WorkerNode(1), kc)
	s.NoError(err)

	s.AssertSomeKubeSystemPods(kc)

	s.T().Log("waiting to see kube-router pods ready")
	s.NoError(common.WaitForKubeRouterReady(s.Context(), kc), "kube-router did not start")

	s.Require().NoError(s.checkCertPerms(s.ControllerNode(0)))
	s.Require().NoError(s.checkCSRs(s.WorkerNode(0), kc))
	s.Require().NoError(s.checkCSRs(s.WorkerNode(1), kc))

	s.Require().NoError(s.verifyKubeletAddressFlag(s.WorkerNode(0)))
	s.Require().NoError(s.verifyKubeletAddressFlag(s.WorkerNode(1)))
	for _, lease := range []string{"kube-scheduler", "kube-controller-manager"} {
		_, err := common.WaitForLease(s.Context(), kc, lease, "kube-system")
		s.Require().NoError(err, lease)
	}

	// We need to first wait till we see pod logs, that's a signal that konnectivity tunnels are up and thus we can then connect to kubelet
	// via the API.
	s.Require().NoError(common.WaitForPodLogs(s.Context(), kc, "kube-system"))
	for i := 0; i < s.WorkerCount; i++ {
		node := s.WorkerNode(i)
		s.T().Logf("checking that we can connect to kubelet metrics on %s", node)
		s.Require().NoError(common.VerifyKubeletMetrics(s.Context(), kc, node))
	}

	s.verifyContainerdDefaultConfig()

	s.verifyCoreDNSAntiAffinity(kc)
}

func (s *BasicSuite) checkCertPerms(node string) error {
	ssh, err := s.SSH(node)
	if err != nil {
		return err
	}
	defer ssh.Disconnect()

	output, err := ssh.ExecWithOutput(s.Context(), `find /var/lib/k0s/custom-data-dir/pki/  \( -name '*.key' -o -name '*.conf' \) -a \! -perm 0640`)
	if err != nil {
		return err
	}

	if output != "" {
		return fmt.Errorf("some private files having non 640 permissions: %s", output)
	}

	return nil
}

// Verifies that kubelet process has the address flag set
func (s *BasicSuite) verifyKubeletAddressFlag(node string) error {
	ssh, err := s.SSH(node)
	if err != nil {
		return err
	}
	defer ssh.Disconnect()

	output, err := ssh.ExecWithOutput(s.Context(), `grep -e '--address=0.0.0.0' /proc/$(pidof kubelet)/cmdline`)
	if err != nil {
		return err
	}
	if output != "--address=0.0.0.0" {
		return fmt.Errorf("kubelet does not have the address flag set")
	}

	return nil
}

func (s *BasicSuite) checkCSRs(node string, kc *kubernetes.Clientset) error {

	return wait.PollImmediate(1*time.Second, 30*time.Second, func() (bool, error) {
		opts := metav1.ListOptions{
			FieldSelector: "spec.signerName=kubernetes.io/kubelet-serving",
		}
		csrs, err := kc.CertificatesV1().CertificateSigningRequests().List(s.Context(), opts)
		if err != nil {
			return false, err
		}

		for _, csr := range csrs.Items {
			if csr.Spec.Username == fmt.Sprintf("system:node:%s", node) {
				if isCSRApproved(csr) {
					return true, nil
				}
			}
		}
		// No approved CSRs found, continue polling
		return false, nil
	})

}

func isCSRApproved(csr certificatesv1.CertificateSigningRequest) bool {
	for _, condition := range csr.Status.Conditions {
		if condition.Type == certificatesv1.CertificateApproved && condition.Reason == "Autoapproved by K0s CSRApprover" {
			return true
		}
	}
	return false
}

func (s *BasicSuite) verifyContainerdDefaultConfig() {
	var defaultConfig bytes.Buffer
	ssh, err := s.SSH(s.WorkerNode(0))
	if !s.NoError(err) {
		return
	}
	defer ssh.Disconnect()

	if !s.NoError(ssh.Exec(s.Context(), "/var/lib/k0s/bin/containerd config default", common.SSHStreams{Out: &defaultConfig})) {
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

func (s *BasicSuite) verifyCoreDNSAntiAffinity(kc *kubernetes.Clientset) {
	opts := metav1.ListOptions{
		LabelSelector: "k8s-app=kube-dns",
	}
	pods, err := kc.CoreV1().Pods("kube-system").List(s.Context(), opts)
	s.NoError(err)
	s.Equal(2, len(pods.Items))
	s.NotEqual(pods.Items[0].Spec.NodeName, pods.Items[1].Spec.NodeName)
}

func TestBasicSuite(t *testing.T) {
	s := BasicSuite{
		common.FootlooseSuite{
			ControllerCount: 1,
			WorkerCount:     2,
		},
	}
	suite.Run(t, &s)
}
