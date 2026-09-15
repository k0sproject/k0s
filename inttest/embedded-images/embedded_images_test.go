// SPDX-FileCopyrightText: 2026 k0s authors
// SPDX-License-Identifier: Apache-2.0

package embeddedimages

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/k0sproject/k0s/inttest/common"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type EmbeddedImagesSuite struct {
	common.BootlooseSuite

	// image is the container image that's embedded into the k0s executable that's
	// being tested.
	image string
}

func (s *EmbeddedImagesSuite) TestEmbeddedImagesAreImported() {
	ctx := s.Context()

	s.Require().NoError(s.InitController(0))
	s.Require().NoError(s.RunWorkers())

	kc, err := s.KubeClient(s.ControllerNode(0))
	s.Require().NoError(err)
	s.Require().NoError(s.WaitForNodeReady(s.WorkerNode(0), kc))

	ssh, err := s.SSH(ctx, s.WorkerNode(0))
	s.Require().NoError(err)
	defer ssh.Disconnect()

	// Check this before anything refers to the image, so that it cannot have been
	// pulled by the kubelet.
	s.T().Log("Verifying that the embedded image has been imported")
	s.assertImageIsPinnedToPayload(ctx, ssh)

	// Embedded bundles are streamed straight out of the k0s executable. Nothing
	// should have been extracted into the OCI bundle directory.
	s.T().Log("Verifying that no bundles have been extracted to disk")
	out, err := ssh.ExecWithOutput(ctx, "ls -A /var/lib/k0s/images")
	s.Require().NoError(err)
	s.Empty(strings.TrimSpace(out), "Expected the OCI bundle directory to be empty")

	// The embedded executables live in the same payload as the image bundles.
	s.T().Log("Verifying that the embedded executables are still staged")
	_, err = ssh.ExecWithOutput(ctx, "/var/lib/k0s/bin/kubelet --version")
	s.Require().NoError(err)

	// An imported image is only useful if it can actually be run. Never pulling it
	// ensures that it's the imported one that's being used.
	s.T().Log("Creating a Pod that must not pull its image")
	pod := corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "embedded"},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{{
				Name:            "embedded",
				Image:           s.image,
				ImagePullPolicy: corev1.PullNever,
				Command:         []string{"sleep", "3600"},
			}},
		},
	}
	_, err = kc.CoreV1().Pods(metav1.NamespaceDefault).Create(ctx, &pod, metav1.CreateOptions{})
	s.Require().NoError(err)
	s.Require().NoError(common.WaitForPod(ctx, kc, "embedded", metav1.NamespaceDefault))

	// Restarting k0s reconciles the embedded bundles once more. The image has to
	// remain pinned: unpinning it would make it eligible for garbage collection,
	// even though the bundle it came from is still embedded.
	s.T().Log("Restarting k0s")
	s.Require().NoError(s.StopWorker(s.WorkerNode(0)))
	// Wait for the node to actually drop out before waiting for it to come back.
	// Its readiness is reported by the kubelet that's just been stopped, so it
	// remains stale for a while, and waiting for it to be ready would return
	// before k0s has been restarted at all.
	s.Require().NoError(
		common.WaitForNodeReadyStatus(ctx, kc, s.WorkerNode(0), corev1.ConditionUnknown),
		"Didn't observe node %s to become non-ready", s.WorkerNode(0),
	)
	s.Require().NoError(s.StartWorker(s.WorkerNode(0)))
	// The kubelet is started after the OCI bundles have been imported, so the
	// node being ready again means that the bundles have been reconciled anew.
	s.Require().NoError(s.WaitForNodeReady(s.WorkerNode(0), kc))

	s.T().Log("Verifying that the embedded image is still pinned")
	s.assertImageIsPinnedToPayload(ctx, ssh)
}

// assertImageIsPinnedToPayload checks that the embedded image is present in
// containerd, that it's pinned, and that it's been imported from the k0s
// executable's ZIP payload rather than from the OCI bundle directory.
func (s *EmbeddedImagesSuite) assertImageIsPinnedToPayload(ctx context.Context, ssh *common.SSHConnection) {
	s.T().Helper()

	out, err := ssh.ExecWithOutput(ctx, fmt.Sprintf(`k0s ctr images ls "name==%s"`, s.image))
	s.Require().NoError(err)
	s.Contains(out, s.image, "Expected the embedded image to be present in containerd")
	s.Contains(out, "io.cri-containerd.pinned=pinned", "Expected the embedded image to be pinned")
	s.Contains(out, "k0s-embedded://images/test-images.tar",
		"Expected the embedded image to be sourced from the k0s executable's payload")
}

func TestEmbeddedImagesSuite(t *testing.T) {
	// Provided by the Makefile, which embeds this image into the k0s executable
	// that's being tested.
	image := os.Getenv("K0S_EMBEDDED_TEST_IMAGE")
	require.NotEmpty(t, image, "K0S_EMBEDDED_TEST_IMAGE is not set")

	suite.Run(t, &EmbeddedImagesSuite{
		BootlooseSuite: common.BootlooseSuite{
			// So that k0s can be restarted.
			LaunchMode:      common.LaunchModeOpenRC,
			ControllerCount: 1,
			WorkerCount:     1,
		},
		image: image,
	})
}
