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
package tunneledkas

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/suite"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	"github.com/k0sproject/k0s/inttest/common"
)

type Suite struct {
	common.FootlooseSuite
}

const config = `
spec:
  api:
    tunneledNetworkingMode: true
`

func (s *Suite) TestK0sTunneledKasMode() {
	s.PutFile(s.ControllerNode(0), "/tmp/k0s.yaml", config)
	s.NoError(s.InitController(0, "--config=/tmp/k0s.yaml"))

	token, err := s.GetJoinToken("worker")
	s.NoError(err)
	s.NoError(s.RunWorkersWithToken(token))

	// out of cluster client
	kc, err := s.KubeClient(s.ControllerNode(0))
	s.NoError(err)

	err = s.WaitForNodeReady(s.WorkerNode(0), kc)
	s.NoError(err)
	err = s.WaitForNodeReady(s.WorkerNode(1), kc)
	s.NoError(err)
	eps, err := kc.CoreV1().Endpoints("default").Get(context.Background(), "kubernetes", v1.GetOptions{})
	s.NoError(err)

	nodes, err := kc.CoreV1().Nodes().List(context.Background(), v1.ListOptions{})
	s.NoError(err)

	s.Assert().Equal(1, len(eps.Subsets))
	s.Assert().Equal(len(nodes.Items), len(eps.Subsets[0].Addresses))

	svc, err := kc.CoreV1().Services("default").Get(context.Background(), "kubernetes", v1.GetOptions{})
	s.NoError(err)
	s.Equal("Local", string(*svc.Spec.InternalTrafficPolicy))

	kubeConfig, err := s.GetKubeConfig(s.ControllerNode(0))
	s.NoError(err)

	// for each node try to call konnectivity-agent directly
	// nodes IPs are not in the config.spec.api.sans
	// so skip x509 verification here for the sake of the test
	kubeConfig.TLSClientConfig.Insecure = true
	kubeConfig.TLSClientConfig.CAData = nil
	for _, addr := range eps.Subsets[0].Addresses {
		kubeConfig.Host = fmt.Sprintf("https://%s:6443", addr.IP)
		nodeLocalClient, err := kubernetes.NewForConfig(kubeConfig)
		s.Require().NoError(err)
		_, err = nodeLocalClient.CoreV1().Nodes().List(context.Background(), v1.ListOptions{})
		s.Require().NoError(err)
	}
}

func TestK0sTunneledKasModeSuite(t *testing.T) {
	s := Suite{
		common.FootlooseSuite{
			ControllerCount: 1,
			WorkerCount:     2,
		},
	}
	suite.Run(t, &s)
}
