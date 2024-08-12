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
	"fmt"
	"strings"
	"testing"

	"github.com/k0sproject/k0s/inttest/common"
	"github.com/k0sproject/k0s/pkg/k0scloudprovider"
	"github.com/k0sproject/k0s/pkg/kubernetes/watch"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"

	"github.com/go-openapi/jsonpointer"
	"github.com/stretchr/testify/suite"
)

type K0sCloudProviderSuite struct {
	common.BootlooseSuite
}

func (s *K0sCloudProviderSuite) TestK0sGetsUp() {
	ctx := s.Context()

	s.Require().NoError(s.InitController(0, "--enable-k0s-cloud-provider", "--k0s-cloud-provider-update-frequency=1s"))
	s.Require().NoError(s.RunWorkers("--enable-cloud-provider"))

	kc, err := s.KubeClient(s.ControllerNode(0))
	s.Require().NoError(err)
	s.Require().NoError(s.WaitJoinAPI(s.ControllerNode(0)))

	nodeName := s.WorkerNode(0)
	err = s.WaitForNodeReady(nodeName, kc)
	s.Require().NoError(err)

	node, err := kc.CoreV1().Nodes().Get(ctx, nodeName, metav1.GetOptions{})
	s.Require().NoError(err)
	s.Equal("k0s-cloud-provider://"+nodeName, node.Spec.ProviderID)

	s.testAddAddress(ctx, kc, nodeName, "1.2.3.4")
	s.testAddAddress(ctx, kc, nodeName, "2041:0000:140F::875B:131B")
	s.testAddAddress(ctx, kc, nodeName, "GIGO")
	s.testAddAddress(ctx, kc, nodeName, "1.2.3.4,GIGO")
}

// testAddAddress adds the provided address to a node and ensures that the
// cloud-provider will set it on the node.
func (s *K0sCloudProviderSuite) testAddAddress(ctx context.Context, client kubernetes.Interface, nodeName string, addresses string) {
	// Adds or sets the special ExternalIPAnnotation with an IP address to the worker
	// node, and after a few seconds the IP address should be listed as a NodeExternalIP.
	patch := fmt.Sprintf(
		`[{"op":"add", "path":"/metadata/annotations/%s", "value":"%s"}]`,
		jsonpointer.Escape(k0scloudprovider.ExternalIPAnnotation), jsonpointer.Escape(addresses),
	)
	_, err := client.CoreV1().Nodes().Patch(ctx, nodeName, types.JSONPatchType, []byte(patch), metav1.PatchOptions{})
	s.Require().NoError(err, "Failed to add or set the annotation for the external IP address")

	// Need to ensure that a matching 'ExternalIP' address has been added,
	// indicating that k0s-cloud-provider properly processed the annotation.
	s.T().Logf("Waiting for k0s-cloud-provider to update the external IP on node %s to %s", nodeName, addresses)
	s.Require().NoError(watch.Nodes(client.CoreV1().Nodes()).
		WithObjectName(nodeName).
		WithErrorCallback(common.RetryWatchErrors(s.T().Logf)).
		Until(ctx, func(node *corev1.Node) (bool, error) {
			for _, addr := range strings.Split(addresses, ",") {
				for _, nodeAddr := range node.Status.Addresses {
					if nodeAddr.Type == corev1.NodeExternalIP {
						if nodeAddr.Address == addr {
							return true, nil
						}
						break
					}
				}
			}

			return false, nil
		}), "While waiting for k0s-cloud-provider to update the external IP on node %s to %s", nodeName, addresses)
}

func TestK0sCloudProviderSuite(t *testing.T) {
	suite.Run(t, &K0sCloudProviderSuite{
		common.BootlooseSuite{
			ControllerCount: 1,
			WorkerCount:     1,
			LaunchMode:      common.LaunchModeOpenRC,
		},
	})
}
