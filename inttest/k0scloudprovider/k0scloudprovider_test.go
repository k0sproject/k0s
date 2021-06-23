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
package k0scloudprovider

import (
	"context"
	"testing"
	"time"

	"github.com/k0sproject/k0s/inttest/common"
	"github.com/k0sproject/k0s/pkg/k0scloudprovider"
	"github.com/stretchr/testify/suite"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

type K0sCloudProviderSuite struct {
	common.FootlooseSuite
}

func (s *K0sCloudProviderSuite) TestK0sGetsUp() {
	s.Require().NoError(s.InitController(0, "--enable-k0s-cloud-provider", "--k0s-cloud-provider-update-frequency=5s"))
	s.Require().NoError(s.RunWorkers("--enable-cloud-provider"))

	kc, err := s.KubeClient(s.ControllerNode(0))
	s.Require().NoError(err)
	s.Require().NoError(s.WaitJoinAPI(s.ControllerNode(0)))

	err = s.WaitForNodeReady(s.WorkerNode(0), kc)
	s.Require().NoError(err)

	// Ensure that worker0 doesn't have the special label ...
	w0labels, err := s.GetNodeLabels(s.WorkerNode(0), kc)
	s.Require().NoError(err)
	s.Require().NotContains(w0labels, k0scloudprovider.ExternalIPLabel)

	// .. and that the external IP has not been defined.
	w0AddrsFoundInitial, err := nodeHasAddressWithType(kc, s.WorkerNode(0), "1.2.3.4", v1.NodeExternalIP)
	s.Require().NoError(err)
	s.Require().False(w0AddrsFoundInitial)

	// Now, add the special 'k0sproject.io/node-ip-external' label with an
	// IP address to the worker node, and after a few seconds the IP address
	// should be listed as an 'ExternalIP'
	w0, err := s.AddNodeLabel(s.WorkerNode(0), kc, k0scloudprovider.ExternalIPLabel, "1.2.3.4")
	s.Require().NoError(err)
	s.Require().NotNil(w0)

	// The k0s-cloud-provider is configured to update every 5s, so wait
	// for 10s and then look for the external IP.
	s.T().Logf("waiting 10s for the next k0s-cloud-provider update")
	time.Sleep(10 * time.Second)

	// Need to ensure that an 'ExternalIP' address of '1.2.3.4' has been added,
	// indicating that k0s-cloud-provider properly processed the label.
	w0AddrsFoundPostUpdate, err := nodeHasAddressWithType(kc, s.WorkerNode(0), "1.2.3.4", v1.NodeExternalIP)
	s.Require().NoError(err)
	s.Require().True(w0AddrsFoundPostUpdate, "unable to find ExternalIP=1.2.3.4")

	// Sanity: Ensure that worker1 doesn't also have the new ExternalIP address
	w1AddrsFound, err := nodeHasAddressWithType(kc, s.WorkerNode(1), "1.2.3.4", v1.NodeExternalIP)
	s.Require().NoError(err)
	s.Require().False(w1AddrsFound, "worker1 incorrectly has ExternalIP=1.2.3.4")
}

// nodeHasAddressWithType is a helper for fetching all of the addresses associated to
// the provided node, and asserting that an IP matches by address + type.
func nodeHasAddressWithType(kc *kubernetes.Clientset, node string, addr string, addrType v1.NodeAddressType) (bool, error) {
	n, err := kc.CoreV1().Nodes().Get(context.TODO(), node, metav1.GetOptions{})
	if err != nil {
		return false, err
	}

	for _, naddr := range n.Status.Addresses {
		if naddr.Type == addrType && naddr.Address == addr {
			return true, nil
		}
	}

	return false, nil
}

func TestK0sCloudProviderSuite(t *testing.T) {
	suite.Run(t, &K0sCloudProviderSuite{
		common.FootlooseSuite{
			ControllerCount: 1,
			WorkerCount:     2,
		},
	})
}
