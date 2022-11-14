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

package network

import (
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/k0sproject/k0s/inttest/common"

	"github.com/stretchr/testify/suite"
	sc "github.com/vmware-tanzu/sonobuoy/pkg/client"
	"github.com/vmware-tanzu/sonobuoy/pkg/dynamic"
	"golang.org/x/mod/semver"
)

const defaultCNI = "kuberouter"

type networkSuite struct {
	common.FootlooseSuite
}

func (s *networkSuite) TestK0sGetsUp() {
	// Which cni to test: kuberouter, calico. Default: kuberouter
	cni := os.Getenv("K0S_NETWORK_CONFORMANCE_CNI")
	if cni == "" {
		cni = defaultCNI
	}
	s.T().Logf("Run conformance tests for CNI: %s", cni)

	s.PutFile(s.ControllerNode(0), "/tmp/k0s.yaml", fmt.Sprintf(k0sConfig, cni))
	s.Require().NoError(s.InitController(0, "--config=/tmp/k0s.yaml", "--disable-components=metrics-server"))
	s.Require().NoError(s.RunWorkers())

	kc, err := s.KubeClient("controller0", "")
	s.Require().NoError(err)

	k8sVersion, err := kc.ServerVersion()
	s.Require().NoError(err)

	err = s.WaitForNodeReady("worker0", kc)
	s.NoError(err)

	err = s.WaitForNodeReady("worker1", kc)
	s.NoError(err)

	s.T().Log("waiting to see kube-proxy pods ready")
	s.NoError(common.WaitForDaemonSet(s.Context(), kc, "kube-proxy"), "kube-proxy did not start")

	restConfig, err := s.GetKubeConfig("controller0")
	s.Require().NoError(err)

	skc, err := dynamic.NewAPIHelperFromRESTConfig(restConfig)
	s.Require().NoError(err)
	client, err := sc.NewSonobuoyClient(restConfig, skc)
	s.Require().NoError(err)

	deadline, _ := s.Context().Deadline()
	err = client.Run(&sc.RunConfig{
		GenConfig: sc.GenConfig{
			EnableRBAC:     true,
			DynamicPlugins: []string{"e2e"},
			PluginEnvOverrides: map[string]map[string]string{
				"e2e": {
					"E2E_FOCUS":         "\\[sig-network\\].*\\[Conformance\\]",
					"E2E_SKIP":          "\\[Serial\\]",
					"E2E_PARALLEL":      "y",
					"E2E_USE_GO_RUNNER": "true",
				},
			},
			KubeVersion: semver.Canonical(k8sVersion.String()),
		},
		Wait:       time.Until(deadline),
		WaitOutput: "Silent",
	})
	s.Require().NoError(err)
	status, err := client.GetStatus(&sc.StatusConfig{Namespace: "sonobuoy"})
	s.Require().NoError(err)

	s.T().Log("sonobuoy test status: ", status)
	s.Require().Equal("complete", status.Status)
	s.Require().Len(status.Plugins, 1)
	s.Require().Equal("passed", status.Plugins[0].ResultStatus)
}

func TestNetworkSuite(t *testing.T) {
	s := networkSuite{
		common.FootlooseSuite{
			ControllerCount: 1,
			WorkerCount:     2,
		},
	}
	suite.Run(t, &s)
}

const k0sConfig = `
spec:
  network:
    provider: %s
`
