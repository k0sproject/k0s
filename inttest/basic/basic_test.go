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
	"fmt"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/k0sproject/k0s/inttest/common"
	capi "k8s.io/api/certificates/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

type BasicSuite struct {
	common.FootlooseSuite
}

func (s *BasicSuite) TestK0sGetsUp() {
	customDataDir := "/var/lib/k0s/custom-data-dir"
	dataDirOpt := fmt.Sprintf("--data-dir=%s", customDataDir)
	s.NoError(s.InitController(0, dataDirOpt))

	token, err := s.GetJoinToken("worker", dataDirOpt)
	s.NoError(err)
	s.NoError(s.RunWorkersWithToken(token, `--labels="k0sproject.io/foo=bar"`, `--kubelet-extra-args=" --address=0.0.0.0  --event-burst=10"`))

	kc, err := s.KubeClient(s.ControllerNode(0), dataDirOpt)
	s.NoError(err)

	err = s.WaitForNodeReady(s.WorkerNode(0), kc)
	s.NoError(err)

	labels, err := s.GetNodeLabels(s.WorkerNode(0), kc)
	s.NoError(err)
	s.Equal("bar", labels["k0sproject.io/foo"])

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

	s.Require().NoError(s.checkCertPerms(s.ControllerNode(0)))
	s.Require().NoError(s.checkCSRs(s.WorkerNode(0), kc))
	s.Require().NoError(s.checkCSRs(s.WorkerNode(1), kc))

	s.Require().NoError(s.verifyKubeletAddressFlag(s.WorkerNode(0)))
	s.Require().NoError(s.verifyKubeletAddressFlag(s.WorkerNode(1)))
	s.Require().NoError(common.WaitForLease(s.Context(), kc, "kube-scheduler", "kube-system"))
	s.Require().NoError(common.WaitForLease(s.Context(), kc, "kube-controller-manager", "kube-system"))
}

func (s *BasicSuite) checkCertPerms(node string) error {
	ssh, err := s.SSH(node)
	if err != nil {
		return err
	}
	defer ssh.Disconnect()

	output, err := ssh.ExecWithOutput(`find /var/lib/k0s/custom-data-dir/pki/  \( -name '*.key' -o -name '*.conf' \) -a \! -perm 0640`)
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

	output, err := ssh.ExecWithOutput(`grep -e '--address=0.0.0.0' /proc/$(pidof kubelet)/cmdline`)
	if err != nil {
		return err
	}
	if output != "--address=0.0.0.0" {
		return fmt.Errorf("kubelet does not have the address flag set")
	}

	return nil
}

func (s *BasicSuite) checkCSRs(node string, kc *kubernetes.Clientset) error {
	opts := v1.ListOptions{
		FieldSelector: "spec.signerName=kubernetes.io/kubelet-serving",
	}
	csrs, err := kc.CertificatesV1().CertificateSigningRequests().List(context.TODO(), opts)
	if err != nil {
		return err
	}

	for _, csr := range csrs.Items {
		if csr.Spec.Username == fmt.Sprintf("system:node:%s", node) {
			if isCSRApproved(csr) {
				return nil
			}
		}
	}
	return fmt.Errorf("no CSRs have been approved")
}

func isCSRApproved(csr capi.CertificateSigningRequest) bool {
	for _, condition := range csr.Status.Conditions {
		if condition.Type == capi.CertificateApproved && condition.Reason == "Autoapproved by K0s CSRApprover" {
			return true
		}
	}
	return false
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
