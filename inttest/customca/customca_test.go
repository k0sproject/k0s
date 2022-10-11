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

package customca

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/k0sproject/k0s/inttest/common"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type CustomCASuite struct {
	common.FootlooseSuite
}

func (s *CustomCASuite) TestK0sGetsUp() {
	// Create an custom certificate to prove that k0s manage to work with it
	ssh, err := s.SSH(s.ControllerNode(0))
	s.NoError(err)
	defer ssh.Disconnect()
	_, err = ssh.ExecWithOutput(s.Context(), "mkdir -p /var/lib/k0s/pki && apk add openssl")
	s.NoError(err)
	_, err = ssh.ExecWithOutput(s.Context(), "openssl genrsa -out /var/lib/k0s/pki/ca.key 2048")
	s.NoError(err)
	_, err = ssh.ExecWithOutput(s.Context(), "openssl req -x509 -new -nodes -key /var/lib/k0s/pki/ca.key -sha256 -days 365 -out /var/lib/k0s/pki/ca.crt -subj \"/CN=Test CA\"")
	s.NoError(err)
	cert, err := ssh.ExecWithOutput(s.Context(), "cat /var/lib/k0s/pki/ca.crt")
	s.NoError(err)

	_, err = ssh.ExecWithOutput(s.Context(), "mkdir -p /var/lib/k0s/manifests/test")
	s.NoError(err)
	ipAddress := s.GetControllerIPAddress(0)
	_, err = ssh.ExecWithOutput(s.Context(), fmt.Sprintf("%s token pre-shared --cert /var/lib/k0s/pki/ca.crt --url https://%s:6443/ --out /var/lib/k0s/manifests/test", s.K0sFullPath, ipAddress))
	s.NoError(err)
	token, err := ssh.ExecWithOutput(s.Context(), "cat /var/lib/k0s/manifests/test/token_* && rm /var/lib/k0s/manifests/test/token_*")
	s.NoError(err)

	s.NoError(s.InitController(0))

	s.NoError(s.RunWorkersWithToken(token))

	kc, err := s.KubeClient(s.ControllerNode(0))
	if err != nil {
		s.FailNow("failed to obtain Kubernetes client", err)
	}

	err = s.WaitForNodeReady(s.WorkerNode(0), kc)
	s.NoError(err)

	pods, err := kc.CoreV1().Pods("kube-system").List(s.Context(), v1.ListOptions{
		Limit: 100,
	})
	s.NoError(err)

	podCount := len(pods.Items)

	s.T().Logf("found %d pods in kube-system", podCount)
	s.Greater(podCount, 0, "expecting to see few pods in kube-system namespace")

	newCert, err := ssh.ExecWithOutput(s.Context(), fmt.Sprintf("cat /var/lib/k0s/pki/ca.crt"))
	s.NoError(err)
	s.Require().Equal(cert, newCert)
}

func TestCustomCASuite(t *testing.T) {
	s := CustomCASuite{
		common.FootlooseSuite{
			ControllerCount: 1,
			WorkerCount:     1,
		},
	}
	suite.Run(t, &s)
}
