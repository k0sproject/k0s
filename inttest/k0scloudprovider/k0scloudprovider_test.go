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

	err = s.WaitForNodeReady(s.WorkerNode(1), kc)
	s.Require().NoError(err)

	// Test the adding of various addresses using addition via annotations
	w0Helper := defaultNodeAddValueHelper(s.AddNodeAnnotation)
	s.testAddAddress(kc, s.WorkerNode(0), "1.2.3.4", w0Helper)
	s.testAddAddress(kc, s.WorkerNode(0), "2041:0000:140F::875B:131B", w0Helper)
	s.testAddAddress(kc, s.WorkerNode(0), "GIGO", w0Helper)
}

// nodeAddValueFunc defines how a key/value can be added to a node
type nodeAddValueFunc func(node string, kc *kubernetes.Clientset, key string, value string) (*v1.Node, error)

// nodeAddValueHelper provides all of the callback functions needed to test
// the addition of addresses into the provider (pre, add, post)
type nodeAddValueHelper struct {
	addressFoundPre  func(kc *kubernetes.Clientset, node string, addr string, addrType v1.NodeAddressType) (bool, error)
	addressAdd       nodeAddValueFunc
	addressFoundPost func(kc *kubernetes.Clientset, node string, addr string, addrType v1.NodeAddressType) (bool, error)
}

// defaultNodeAddValueHelper creates a nodeAddValueHelper using the provided
// adder function (ie. labels, or annotation add functions)
func defaultNodeAddValueHelper(adder nodeAddValueFunc) nodeAddValueHelper {
	return nodeAddValueHelper{
		addressFoundPre:  nodeHasAddressWithType,
		addressAdd:       adder,
		addressFoundPost: nodeHasAddressWithType,
	}
}

// testAddAddress adds the provided address to a node via a helper. This ensures that
// the address doesn't already exist, can be added successfully, and exists after addition.
func (s *K0sCloudProviderSuite) testAddAddress(kc *kubernetes.Clientset, node string, addr string, helper nodeAddValueHelper) {
	s.T().Logf("Testing add address - node=%s, addr=%s", node, addr)

	addrFound, err := helper.addressFoundPre(kc, node, addr, v1.NodeExternalIP)
	s.Require().NoError(err)
	s.Require().False(addrFound, "ExternalIP=%s already exists on node=%s", addr, node)

	// Now, add the special 'k0sproject.io/node-ip-external' key with an IP address to the
	// worker node, and after a few seconds the IP address should be listed as an 'ExternalIP'
	w, err := helper.addressAdd(node, kc, k0scloudprovider.ExternalIPAnnotation, addr)
	s.Require().NoError(err)
	s.Require().NotNil(w)

	// The k0s-cloud-provider is configured to update every 5s, so wait for 10s and then
	// look for the external IP.
	s.T().Logf("waiting 10s for the next k0s-cloud-provider update (testAddAddress: node=%s, addr=%s)", node, addr)
	time.Sleep(10 * time.Second)

	// Need to ensure that a matching 'ExternalIP' address has been added, indicating that
	// k0s-cloud-provider properly processed the annotation.
	foundPostUpdate, err := helper.addressFoundPost(kc, node, addr, v1.NodeExternalIP)
	s.Require().NoError(err)
	s.Require().True(foundPostUpdate, "unable to find ExternalIP=%s on node=%s", addr, node)
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
