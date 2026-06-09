// SPDX-FileCopyrightText: 2026 k0s authors
// SPDX-License-Identifier: Apache-2.0

package patches

import (
	"testing"

	"github.com/k0sproject/k0s/inttest/common"

	"github.com/stretchr/testify/suite"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type PatchesSuite struct {
	common.BootlooseSuite
}

// k0sConfig runs the cluster with Calico as the CNI and patches three generated
// resources, each with a different patch type:
//   - the CoreDNS Deployment, via a strategic merge patch adding a label
//   - the metrics-server Deployment, via an RFC 6902 JSON patch adding a label
//   - the calico-node DaemonSet, via an RFC 7386 JSON merge patch adding a label
const k0sConfig = `
apiVersion: k0s.k0sproject.io/v1beta1
kind: ClusterConfig
metadata:
  name: k0s
spec:
  network:
    provider: calico
    coreDNS:
      patches:
        - target:
            kind: Deployment
            name: coredns
            namespace: kube-system
          patch:
            type: strategic
            content: |
              metadata:
                labels:
                  k0s.k0sproject.io/patched: coredns
    calico:
      patches:
        - target:
            kind: DaemonSet
            name: calico-node
            namespace: kube-system
          patch:
            type: merge
            content: |
              {"metadata": {"labels": {"patched": "calico-node"}}}
  metricsServer:
    patches:
      - target:
          kind: Deployment
          name: metrics-server
          namespace: kube-system
        patch:
          type: json
          content: |
            [{"op": "add", "path": "/metadata/labels/patched", "value": "metrics-server"}]
`

func (s *PatchesSuite) TestK0sGetsUp() {
	s.PutFile(s.ControllerNode(0), "/tmp/k0s.yaml", k0sConfig)
	s.NoError(s.InitController(0, "--config=/tmp/k0s.yaml"))
	s.NoError(s.RunWorkers())

	kc, err := s.KubeClient(s.ControllerNode(0))
	s.Require().NoError(err)

	// With Calico as the CNI, the worker won't become ready until calico-node
	// is running on it, so this also gates on Calico being up.
	s.NoError(s.WaitForNodeReady(s.WorkerNode(0), kc))

	s.T().Log("waiting for CoreDNS to become ready")
	s.Require().NoError(common.WaitForCoreDNSReady(s.Context(), kc))

	cfg, err := s.GetKubeConfig(s.ControllerNode(0))
	s.Require().NoError(err)
	s.T().Log("waiting for metrics-server to become ready")
	s.Require().NoError(common.WaitForMetricsReady(s.Context(), cfg))

	s.T().Log("verifying that the CoreDNS Deployment was patched (strategic merge)")
	coredns, err := kc.AppsV1().Deployments(metav1.NamespaceSystem).Get(s.Context(), "coredns", metav1.GetOptions{})
	s.Require().NoError(err)
	s.Equal("coredns", coredns.Labels["k0s.k0sproject.io/patched"],
		"expected the CoreDNS Deployment to carry the patched label")

	s.T().Log("verifying that the metrics-server Deployment was patched (json patch)")
	metricsServer, err := kc.AppsV1().Deployments(metav1.NamespaceSystem).Get(s.Context(), "metrics-server", metav1.GetOptions{})
	s.Require().NoError(err)
	s.Equal("metrics-server", metricsServer.Labels["patched"],
		"expected the metrics-server Deployment to carry the patched label")

	s.T().Log("verifying that the calico-node DaemonSet was patched (merge patch)")
	calicoNode, err := kc.AppsV1().DaemonSets(metav1.NamespaceSystem).Get(s.Context(), "calico-node", metav1.GetOptions{})
	s.Require().NoError(err)
	s.Equal("calico-node", calicoNode.Labels["patched"],
		"expected the calico-node DaemonSet to carry the patched label")
}

func TestPatchesSuite(t *testing.T) {
	s := PatchesSuite{
		common.BootlooseSuite{
			ControllerCount: 1,
			WorkerCount:     1,
		},
	}
	suite.Run(t, &s)
}
