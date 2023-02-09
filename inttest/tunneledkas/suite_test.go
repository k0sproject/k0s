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

package tunneledkas

import (
	"fmt"
	"net"
	"net/url"
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
    port: 7443
    tunneledNetworkingMode: true
`

func (s *Suite) TestK0sTunneledKasMode() {
	s.PutFile(s.ControllerNode(0), "/tmp/k0s.yaml", config)
	s.NoError(s.InitController(0, "--config=/tmp/k0s.yaml"))

	token, err := s.GetJoinToken("worker")
	s.Require().NoError(err)
	s.NoError(s.RunWorkersWithToken(token))

	// out of cluster client
	kc, err := s.KubeClient(s.ControllerNode(0))
	s.Require().NoError(err)

	err = s.WaitForNodeReady(s.WorkerNode(0), kc)
	s.NoError(err)
	err = s.WaitForNodeReady(s.WorkerNode(1), kc)
	s.NoError(err)

	s.Run("Services", func() {
		require := s.Require()
		s.T().Parallel()

		svc, err := kc.CoreV1().Services("default").Get(s.Context(), "kubernetes", v1.GetOptions{})
		require.NoError(err)
		require.Equal("Local", string(*svc.Spec.InternalTrafficPolicy))
	})

	s.Run("Nodes", func() {
		require := s.Require()
		s.T().Parallel()

		nodes, err := kc.CoreV1().Nodes().List(s.Context(), v1.ListOptions{})
		require.NoError(err)
		require.Len(nodes.Items, s.WorkerCount)
	})

	workerIPs := make([]string, s.WorkerCount)
	for i := range workerIPs {
		workerIPs[i] = s.GetWorkerIPAddress(i)
	}

	s.Run("Endpoints", func() {
		require := s.Require()
		s.T().Parallel()

		eps, err := kc.CoreV1().Endpoints("default").Get(s.Context(), "kubernetes", v1.GetOptions{})
		require.NoError(err)
		require.Len(eps.Subsets, 1)
		subsetIPs := make([]string, 0, len(eps.Subsets[0].Addresses))
		for _, addr := range eps.Subsets[0].Addresses {
			subsetIPs = append(subsetIPs, addr.IP)
		}
		require.ElementsMatch(workerIPs, subsetIPs)
	})

	// for each node try to call konnectivity-agent directly
	// nodes IPs are not in the config.spec.api.sans
	// so skip x509 verification here for the sake of the test
	s.Run("Konnectivity", func() {
		kubeConfig, err := s.GetKubeConfig(s.ControllerNode(0))
		s.Require().NoError(err)
		kubeConfig.TLSClientConfig.Insecure = true
		kubeConfig.TLSClientConfig.CAData = nil

		for i, ip := range workerIPs {
			ip, kubeConfig := ip, *kubeConfig
			s.Run(fmt.Sprintf("worker%d", i), func() {
				require := s.Require()
				s.T().Parallel()

				kubeConfig.Host = (&url.URL{Scheme: "https", Host: net.JoinHostPort(ip, "6443")}).String()
				nodeLocalClient, err := kubernetes.NewForConfig(&kubeConfig)
				require.NoError(err)
				_, err = nodeLocalClient.CoreV1().Nodes().List(s.Context(), v1.ListOptions{})
				require.NoError(err)
			})
		}
	})
}

func TestK0sTunneledKasModeSuite(t *testing.T) {
	s := Suite{
		common.FootlooseSuite{
			ControllerCount:     1,
			WorkerCount:         2,
			KubeAPIExternalPort: 7443,
		},
	}
	suite.Run(t, &s)
}
