// SPDX-FileCopyrightText: 2023 k0s authors
// SPDX-License-Identifier: Apache-2.0

package containerdimports

import (
	"testing"

	"github.com/k0sproject/k0s/inttest/common"

	"github.com/stretchr/testify/suite"

	corev1 "k8s.io/api/core/v1"
	nodesv1 "k8s.io/api/node/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type ContainerdImportsSuite struct {
	common.BootlooseSuite
}

func (s *ContainerdImportsSuite) TestK0sGetsUp() {
	ctx := s.Context()
	ssh, err := s.SSH(ctx, s.ControllerNode(0))
	s.Require().NoError(err)
	defer ssh.Disconnect()

	s.Require().NoError(s.InitController(0))

	s.Require().NoError(s.RunWorkers())

	kc, err := s.KubeClient(s.ControllerNode(0))
	if err != nil {
		s.FailNow("failed to obtain Kubernetes client", err)
	}

	s.Require().NoError(s.WaitForNodeReady(s.WorkerNode(0), kc))

	s.AssertSomeKubeSystemPods(kc)

	s.T().Log("waiting to see kube-router pods ready")
	s.NoError(common.WaitForKubeRouterReady(ctx, kc), "kube-router did not start")

	s.addContainerdRuntime()
	s.T().Log("Creating new RuntimeClass for foo runtime")
	runtimeClassName := "foo"

	runtimeClass := nodesv1.RuntimeClass{
		ObjectMeta: metav1.ObjectMeta{
			Name: runtimeClassName,
		},
		Handler: runtimeClassName,
	}
	_, err = kc.NodeV1().RuntimeClasses().Create(ctx, &runtimeClass, metav1.CreateOptions{})
	s.Require().NoError(err)
	s.T().Log("Creating new Pod for foo runtime")
	// Create new Pod for foo runtime
	pod := corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name: "foo",
		},
		Spec: corev1.PodSpec{
			RuntimeClassName: &runtimeClassName,
			Containers: []corev1.Container{
				{
					Name:  "foo",
					Image: "docker.io/library/nginx:1.29.4-alpine",
				},
			},
		},
	}
	_, err = kc.CoreV1().Pods(metav1.NamespaceDefault).Create(ctx, &pod, metav1.CreateOptions{})
	s.Require().NoError(err)
	s.Require().NoError(common.WaitForPod(ctx, kc, "foo", metav1.NamespaceDefault))

	s.T().Log("Creating new Pod for default runc runtime")
	normalNginxPod := corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name: "normal-nginx",
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:  "nginx",
					Image: "docker.io/library/nginx:1.29.4-alpine",
				},
			},
		},
	}
	_, err = kc.CoreV1().Pods(metav1.NamespaceDefault).Create(ctx, &normalNginxPod, metav1.CreateOptions{})
	s.Require().NoError(err)
	s.Require().NoError(common.WaitForPod(ctx, kc, "normal-nginx", metav1.NamespaceDefault))

}

func (s *ContainerdImportsSuite) addContainerdRuntime() {
	ctx := s.Context()
	s.T().Log("Setting up alternative runtime and config")
	workerSSH, err := s.SSH(ctx, s.WorkerNode(0))
	s.Require().NoError(err)
	defer workerSSH.Disconnect()

	// Make an "alias" runtime using runc
	s.Require().NoError(workerSSH.Exec(ctx, "ln -s /var/lib/k0s/bin/runc /var/lib/k0s/bin/runfoo", common.SSHStreams{}))

	// Configure containerd to use it
	s.PutFile(s.WorkerNode(0), "/etc/k0s/containerd.d/foo.toml", fooRuntimeConfig)
}

func TestContainerdImportsSuite(t *testing.T) {
	s := ContainerdImportsSuite{
		common.BootlooseSuite{
			LaunchMode:      common.LaunchModeOpenRC, // so we can easily restart k0s
			ControllerCount: 1,
			WorkerCount:     1,
		},
	}
	suite.Run(t, &s)
}

const fooRuntimeConfig = `
version = 3

[plugins."io.containerd.cri.v1.runtime".containerd.runtimes.foo]
      runtime_type = "io.containerd.runc.v2"
      [plugins."io.containerd.cri.v1.runtime".containerd.runtimes.foo.options]
        BinaryName = "/var/lib/k0s/bin/runfoo"
`
