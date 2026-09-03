// SPDX-FileCopyrightText: 2026 k0s authors
// SPDX-License-Identifier: Apache-2.0

package manifestrestart

import (
	"context"
	"testing"

	"github.com/k0sproject/k0s/inttest/common"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"

	"github.com/stretchr/testify/suite"
)

type ManifestRestartSuite struct {
	common.BootlooseSuite
}

// TestManifestDirNotPruned reproduces the scenario from
// https://github.com/k0sproject/k0s/issues/8214: if a controller loses its
// local manifest directory (e.g. due to ephemeral storage on a pod restart)
// while the Kubernetes objects owned by those manifests still exist, the
// applier-manager must not treat the (transiently) empty stack directories
// as an implicit request to prune the resources it previously applied.
func (s *ManifestRestartSuite) TestManifestDirNotPruned() {
	ctx := s.Context()

	s.Require().NoError(s.InitController(0))
	s.Require().NoError(s.RunWorkers())

	kc, err := s.KubeClient(s.ControllerNode(0))
	s.Require().NoError(err)

	s.Require().NoError(s.WaitForNodeReady(s.ControllerNode(0), kc))
	for i := range s.WorkerCount {
		s.Require().NoError(s.WaitForNodeReady(s.WorkerNode(i), kc))
	}

	s.Require().NoError(common.WaitForCoreDNSReady(ctx, kc))
	s.Require().NoError(common.WaitForDeployment(ctx, kc, "metrics-server", metav1.NamespaceSystem))

	coreDNSUID := s.deploymentUID(ctx, kc, "coredns")
	metricsServerUID := s.deploymentUID(ctx, kc, "metrics-server")

	s.Require().NoError(s.StopController(s.ControllerNode(0)))

	ssh, err := s.SSH(ctx, s.ControllerNode(0))
	s.Require().NoError(err)
	defer ssh.Disconnect()
	// Simulate the loss of local, ephemeral state (e.g. a fresh emptyDir on
	// pod restart) while the Kubernetes objects owned by the previous
	// process instance still exist.
	_, err = ssh.ExecWithOutput(ctx, "rm -rf /var/lib/k0s/manifests")
	s.Require().NoError(err)

	s.Require().NoError(s.StartController(s.ControllerNode(0)))
	s.Require().NoError(s.WaitForKubeAPI(s.ControllerNode(0)))
	s.Require().NoError(s.WaitForNodeReady(s.ControllerNode(0), kc))

	s.Require().NoError(common.WaitForCoreDNSReady(ctx, kc))
	s.Require().NoError(common.WaitForDeployment(ctx, kc, "metrics-server", metav1.NamespaceSystem))

	s.Equal(coreDNSUID, s.deploymentUID(ctx, kc, "coredns"), "coredns Deployment must not have been deleted and re-created")
	s.Equal(metricsServerUID, s.deploymentUID(ctx, kc, "metrics-server"), "metrics-server Deployment must not have been deleted and re-created")
}

func (s *ManifestRestartSuite) deploymentUID(ctx context.Context, kc kubernetes.Interface, name string) types.UID {
	d, err := kc.AppsV1().Deployments(metav1.NamespaceSystem).Get(ctx, name, metav1.GetOptions{})
	s.Require().NoError(err)
	return d.UID
}

func TestManifestRestartSuite(t *testing.T) {
	s := ManifestRestartSuite{
		common.BootlooseSuite{
			ControllerCount: 1,
			WorkerCount:     1,
		},
	}
	suite.Run(t, &s)
}
