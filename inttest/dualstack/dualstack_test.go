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

package dualstack

// Package implements basic smoke test for dualstack setup.
// Since we run tests under containers environment in the GHA we can't
// actually check proper network connectivity.
// Until wi migrate toward VM based test suites
// this test only checks that nodes in the cluster
// have proper values for spec.PodCIDRs

import (
	"fmt"
	"net"
	"os"
	"strings"

	"github.com/stretchr/testify/suite"
	v1meta "k8s.io/apimachinery/pkg/apis/meta/v1"

	"testing"

	"time"

	"github.com/k0sproject/k0s/inttest/common"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8s "k8s.io/client-go/kubernetes"

	"context"

	"k8s.io/apimachinery/pkg/util/wait"
)

type DualstackSuite struct {
	common.BootlooseSuite

	client *k8s.Clientset
}

func (s *DualstackSuite) TestDualStackNodesHavePodCIDRs() {
	nl, err := s.client.CoreV1().Nodes().List(s.Context(), v1meta.ListOptions{})
	s.Require().NoError(err)
	for _, n := range nl.Items {
		s.Require().Len(n.Spec.PodCIDRs, 2, "Each node must have ipv4 and ipv6 pod cidr")
	}
}

func (s *DualstackSuite) TestDualStackControlPlaneComponentsHaveServiceCIDRs() {
	const expected = "--service-cluster-ip-range=10.96.0.0/12,fd01::/108"
	node := s.ControllerNode(0)

	s.Contains(s.cmdlineForExecutable(node, "kube-apiserver"), expected)
	s.Contains(s.cmdlineForExecutable(node, "kube-controller-manager"), expected)
}

func (s *DualstackSuite) cmdlineForExecutable(node, binary string) []string {
	require := s.Require()
	ssh, err := s.SSH(s.Context(), node)
	require.NoError(err)
	defer ssh.Disconnect()

	output, err := ssh.ExecWithOutput(s.Context(), fmt.Sprintf("pidof -- %q", binary))
	require.NoError(err)

	pids := strings.Split(output, " ")
	require.Len(pids, 1, "Expected a single pid")

	output, err = ssh.ExecWithOutput(s.Context(), fmt.Sprintf("cat /proc/%q/cmdline", pids[0]))
	require.NoError(err, "Failed to get cmdline for PID %s", pids[0])
	return strings.Split(output, "\x00")
}

func (s *DualstackSuite) SetupSuite() {
	isDockerIPv6Enabled, err := s.IsDockerIPv6Enabled()
	s.NoError(err)
	s.Require().True(isDockerIPv6Enabled, "Please enable IPv6 in docker before running this test")
	s.BootlooseSuite.SetupSuite()

	k0sConfig := k0sConfigWithCalicoDualStack

	if os.Getenv("K0S_NETWORK") == "kube-router" {
		s.T().Log("Using kube-router network")
		k0sConfig = k0sConfigWithKuberouterDualStack
	}
	s.PutFile(s.ControllerNode(0), "/tmp/k0s.yaml", k0sConfig)
	controllerArgs := []string{"--config=/tmp/k0s.yaml"}
	if os.Getenv("K0S_ENABLE_DYNAMIC_CONFIG") == "true" {
		s.T().Log("Enabling dynamic config for controller")
		controllerArgs = append(controllerArgs, "--enable-dynamic-config")
	}
	s.Require().NoError(s.InitController(0, controllerArgs...))
	s.Require().NoError(s.RunWorkers())
	client, err := s.KubeClient(s.ControllerNode(0))
	s.Require().NoError(err)
	err = s.WaitForNodeReady(s.WorkerNode(0), client)
	s.Require().NoError(err)

	err = s.WaitForNodeReady(s.WorkerNode(1), client)
	s.Require().NoError(err)

	for i := 0; i < s.WorkerCount; i++ {
		ssh, err := s.SSH(s.Context(), s.WorkerNode(i))
		s.Require().NoError(err)
		defer ssh.Disconnect()
		output, err := ssh.ExecWithOutput(s.Context(), "cat /proc/sys/net/ipv6/conf/all/disable_ipv6")
		s.Require().NoError(err)
		s.T().Logf("worker%d: /proc/sys/net/ipv6/conf/all/disable_ipv6=%s", i, output)
	}

	kc, err := s.KubeClient("controller0", "")
	s.Require().NoError(err)
	restConfig, err := s.GetKubeConfig("controller0", "")
	s.Require().NoError(err)

	createdTargetPod, err := kc.CoreV1().Pods("default").Create(s.Context(), &corev1.Pod{
		TypeMeta:   metav1.TypeMeta{Kind: "Pod", APIVersion: "v1"},
		ObjectMeta: metav1.ObjectMeta{Name: "nginx-worker0"},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{{Name: "nginx-worker0", Image: "docker.io/library/nginx:1.23.1-alpine"}},
			NodeSelector: map[string]string{
				"kubernetes.io/hostname": "worker0",
			},
		},
	}, metav1.CreateOptions{})
	s.Require().NoError(err)
	s.Require().NoError(common.WaitForPod(s.Context(), kc, "nginx-worker0", "default"), "nginx-worker0 pod did not start")

	targetPod, err := kc.CoreV1().Pods(createdTargetPod.Namespace).Get(s.Context(), createdTargetPod.Name, metav1.GetOptions{})
	s.Require().NoError(err)

	sourcePod, err := kc.CoreV1().Pods("default").Create(s.Context(), &corev1.Pod{
		TypeMeta:   metav1.TypeMeta{Kind: "Pod", APIVersion: "v1"},
		ObjectMeta: metav1.ObjectMeta{Name: "nginx-worker1"},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{{Name: "alpine", Image: "docker.io/library/nginx:1.23.1-alpine"}},
			NodeSelector: map[string]string{
				"kubernetes.io/hostname": "worker1",
			},
		},
	}, metav1.CreateOptions{})
	s.Require().NoError(err)
	s.NoError(common.WaitForPod(s.Context(), kc, "nginx-worker1", "default"), "nginx-worker1 pod did not start")

	// test ipv6 address
	err = wait.PollImmediateWithContext(s.Context(), 100*time.Millisecond, time.Minute, func(ctx context.Context) (done bool, err error) {
		s.Require().Equal(len(targetPod.Status.PodIPs), 2)
		podIP := targetPod.Status.PodIPs[1].IP
		targetIP := net.ParseIP(podIP)
		s.Require().NotNil(targetIP)
		out, err := common.PodExecCmdOutput(kc, restConfig, sourcePod.Name, sourcePod.Namespace, fmt.Sprintf("/usr/bin/wget -qO- %s", targetIP))
		s.T().Log(out, err)
		if err != nil {
			s.T().Log("error calling ipv6 address: ", err)
			return false, nil
		}
		s.T().Log("server response", out)
		return strings.Contains(out, "Welcome to nginx"), nil
	})
	s.Require().NoError(err)

	// test ipv4 address
	err = wait.PollImmediateWithContext(s.Context(), 100*time.Millisecond, time.Minute, func(ctx context.Context) (done bool, err error) {
		s.Require().Equal(len(targetPod.Status.PodIPs), 2)
		podIP := targetPod.Status.PodIPs[0].IP
		targetIP := net.ParseIP(podIP)
		s.Require().NotNil(targetIP)
		out, err := common.PodExecCmdOutput(kc, restConfig, sourcePod.Name, sourcePod.Namespace, fmt.Sprintf("/usr/bin/wget -qO- %s", targetIP))
		s.T().Log(out, err)
		if err != nil {
			s.T().Log("error calling ipv4 address: ", err)
			return false, nil
		}
		s.T().Log("server response", out)
		return strings.Contains(out, "Welcome to nginx"), nil
	})
	s.Require().NoError(err)
	s.client = client
}

func TestDualStack(t *testing.T) {

	s := DualstackSuite{
		common.BootlooseSuite{
			ControllerCount: 1,
			WorkerCount:     2,
		},
		nil,
	}

	suite.Run(t, &s)

}

const k0sConfigWithCalicoDualStack = `
spec:
  network:
    kubeProxy:
      mode: ipvs
    provider: calico
    calico:
      mode: "bird"
    dualStack:
      enabled: true
      IPv6podCIDR: "fd00::/108"
      IPv6serviceCIDR: "fd01::/108"
    podCIDR: 10.244.0.0/16
    serviceCIDR: 10.96.0.0/12`

const k0sConfigWithKuberouterDualStack = `
spec:
  network:
    provider: kuberouter
    dualStack:
      enabled: true
      IPv6podCIDR: "fd00::/108"
      IPv6serviceCIDR: "fd01::/108"
    podCIDR: 10.244.0.0/16
    serviceCIDR: 10.96.0.0/12`
