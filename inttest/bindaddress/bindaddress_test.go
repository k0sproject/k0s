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
package bindaddress

import (
	"bytes"
	"fmt"
	"testing"
	"text/template"

	"github.com/stretchr/testify/suite"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/k0sproject/k0s/inttest/common"
)

type BindAddressSuite struct {
	common.FootlooseSuite
}

var config = `
spec:
  bindAddress: {{ .BindAddress}}
`

func (s *BindAddressSuite) getControllerConfig(bindAddress string) string {
	data := struct {
		BindAddress string
	}{
		BindAddress: bindAddress,
	}
	content := bytes.NewBuffer([]byte{})
	s.Require().NoError(template.Must(template.New("k0s.yaml").Parse(config)).Execute(content, data), "can't execute k0s.yaml template")
	return content.String()

}

func (s *BindAddressSuite) TestK0sGetsUp() {
	controllerIP := s.GetControllerIPAddress(0)
	config := s.getControllerConfig(controllerIP)
	s.PutFile("controller0", "/tmp/k0s.yaml", config)
	s.Require().NoError(s.InitController(0, "--config=/tmp/k0s.yaml"))

	token, err := s.GetJoinToken("worker")
	s.NoError(err)
	s.NoError(s.RunWorkersWithToken(token))

	kc, err := s.KubeClient(s.ControllerNode(0))
	if err != nil {
		s.FailNow("failed to obtain Kubernetes client", err)
	}

	err = s.WaitForNodeReady(s.WorkerNode(0), kc)
	s.NoError(err)

	err = s.WaitForNodeReady(s.WorkerNode(1), kc)
	s.NoError(err)

	pods, err := kc.CoreV1().Pods("kube-system").List(s.Context(), v1.ListOptions{
		Limit: 100,
	})
	s.NoError(err)

	podCount := len(pods.Items)

	s.T().Logf("found %d pods in kube-system", podCount)
	s.Greater(podCount, 0, "expecting to see few pods in kube-system namespace")

	s.T().Log("waiting to see kube-router pods ready")
	s.NoError(common.WaitForKubeRouterReadyWithContext(s.Context(), kc), "kube-router did not start")

	s.Require().NoError(common.WaitForLease(s.Context(), kc, "kube-scheduler", "kube-system"))
	s.Require().NoError(common.WaitForLease(s.Context(), kc, "kube-controller-manager", "kube-system"))

	s.VerifyProcessFlagsContains("controller0", "/var/lib/k0s/bin/kube-apiserver", fmt.Sprintf("--bind-address=%s", controllerIP))

	s.VerifyProcessFlagsContains("controller0", "k0s api", fmt.Sprintf("--bind-address=%s", controllerIP))

	s.VerifyProcessFlagsContains("controller0", "/var/lib/k0s/bin/konnectivity-server", fmt.Sprintf("--agent-bind-address=%s", controllerIP))
}

func TestBindAddressSuite(t *testing.T) {
	s := BindAddressSuite{
		common.FootlooseSuite{
			ControllerCount: 1,
			WorkerCount:     2,
		},
	}
	suite.Run(t, &s)
}
