// SPDX-FileCopyrightText: 2026 k0s authors
// SPDX-License-Identifier: Apache-2.0

package containerd_deprecations

import (
	"testing"
	"time"

	"github.com/k0sproject/k0s/inttest/common"
	"github.com/stretchr/testify/suite"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type ContainerdDeprecationsSuite struct {
	common.BootlooseSuite
}

func (s *ContainerdDeprecationsSuite) TestContainerdDeprecationMonitor() {
	ctx := s.Context()

	s.T().Log("Adding deprecated containerd configuration")
	s.addDeprecatedContainerdConfig()

	s.Require().NoError(s.InitController(0))
	s.Require().NoError(s.RunWorkers())

	kc, err := s.KubeClient(s.ControllerNode(0))
	s.Require().NoError(err)

	s.Require().NoError(s.WaitForNodeReady(s.WorkerNode(0), kc))

	s.T().Log("Checking for deprecation condition")
	var foundCondition bool
	s.Eventually(func() bool {
		node, err := kc.CoreV1().Nodes().Get(ctx, s.WorkerNode(0), metav1.GetOptions{})
		if err != nil {
			s.T().Logf("Failed to get node: %v", err)
			return false
		}

		s.T().Logf("Node has %d conditions", len(node.Status.Conditions))
		for _, cond := range node.Status.Conditions {
			s.T().Logf("Condition: Type=%s, Status=%s, Reason=%s, Message=%s",
				cond.Type, cond.Status, cond.Reason, cond.Message)
			if cond.Type == "ContainerdHasNoDeprecations" {
				foundCondition = true
				return cond.Status == corev1.ConditionTrue
			}
		}
		return false
	}, 10*time.Minute, 10*time.Second, "Expected ContainerdHasNoDeprecations to be False")

	s.Require().True(foundCondition, "ContainerdHasNoDeprecations condition was never created")

	/* Containerd only has one deprecated option which is enabled CDI. Unfortunately
	 * if it's explicitly disabled there is no deprecation warning because containerd
	 * overrides it to true, so we can't check for events.
	s.T().Log("Checking for deprecation events")
	s.Eventually(func() bool {
		events, err := kc.CoreV1().Events(metav1.NamespaceDefault).List(ctx, metav1.ListOptions{
			FieldSelector: "involvedObject.name=" + s.WorkerNode(0),
		})
		if err != nil {
			return false
		}

		for _, event := range events.Items {
			if event.Reason == "ContainerdDeprecationDetected" {
				s.T().Logf("Found event: %s", event.Message)
				return true
			}
		}
		return false
	}, 2*time.Minute, 5*time.Second, "Expected ContainerdDeprecationDetected event")
	*/
}

func (s *ContainerdDeprecationsSuite) addDeprecatedContainerdConfig() {
	ssh, err := s.SSH(s.Context(), s.WorkerNode(0))
	s.Require().NoError(err)
	defer ssh.Disconnect()

	s.Require().NoError(ssh.Exec(s.Context(), "mkdir -p /etc/k0s/containerd.d", common.SSHStreams{}))
	s.PutFile(s.WorkerNode(0), "/etc/k0s/containerd.d/deprecated.toml", deprecatedConfig)
}

func TestContainerdDeprecationsSuite(t *testing.T) {
	s := ContainerdDeprecationsSuite{
		common.BootlooseSuite{
			LaunchMode:      common.LaunchModeOpenRC,
			ControllerCount: 1,
			WorkerCount:     1,
		},
	}
	suite.Run(t, &s)
}

// TODO before 1.36: This is broken at the moment, we have to figure out
// how to merge the configuration and revist this test.
const deprecatedConfig = `version = 2

[plugins]
  [plugins."io.containerd.grpc.v1.cri"]
    [plugins."io.containerd.grpc.v1.cri".registry]
      [plugins."io.containerd.grpc.v1.cri".registry.mirrors]
        [plugins."io.containerd.grpc.v1.cri".registry.mirrors."docker.io"]
          endpoint = ["https://registry-1.docker.io"]
    [plugins."io.containerd.grpc.v1.cri".cni]
      bin_dir = "/opt/cni/bin"
`
