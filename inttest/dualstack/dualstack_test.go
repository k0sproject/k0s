// SPDX-FileCopyrightText: 2021 k0s authors
// SPDX-License-Identifier: Apache-2.0

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

	client      *k8s.Clientset
	defaultIPv6 bool
}

func (s *DualstackSuite) TestDualStackNodesHavePodCIDRs() {
	nl, err := s.client.CoreV1().Nodes().List(s.Context(), metav1.ListOptions{})
	s.Require().NoError(err)
	for _, n := range nl.Items {
		s.Require().Len(n.Spec.PodCIDRs, 2, "Each node must have ipv4 and ipv6 pod cidr")
	}
}

func (s *DualstackSuite) TestDualStackControlPlaneComponentsHaveServiceCIDRs() {
	const expectedIPv4 = "--service-cluster-ip-range=10.96.0.0/12,fd01::/108"
	const expectedIPv6 = "--service-cluster-ip-range=fd01::/108,10.96.0.0/12"
	node := s.ControllerNode(0)

	expected := expectedIPv4
	if s.defaultIPv6 {
		expected = expectedIPv6
	}
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
	require.NoErrorf(err, "Failed to get cmdline for PID %s", pids[0])
	return strings.Split(output, "\x00")
}

func (s *DualstackSuite) SetupSuite() {
	isDockerIPv6Enabled, err := s.IsDockerIPv6Enabled()
	s.NoError(err)
	s.Require().True(isDockerIPv6Enabled, "Please enable IPv6 in docker before running this test")
	s.BootlooseSuite.SetupSuite()

	target := os.Getenv("K0S_INTTEST_TARGET")

	k0sConfig := k0sConfigWithCalicoDualStack

	if strings.Contains(target, "kuberouter") {
		s.T().Log("Using kube-router network")
		ipv6Address := s.getIPv6Address(s.ControllerNode(0))
		k0sConfig = fmt.Sprintf(k0sConfigWithKuberouterDualStack, ipv6Address)
		s.defaultIPv6 = true
	}
	s.PutFile(s.ControllerNode(0), "/tmp/k0s.yaml", k0sConfig)
	controllerArgs := []string{"--config=/tmp/k0s.yaml"}
	if strings.Contains(os.Getenv("K0S_INTTEST_TARGET"), "dynamicconfig") {
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

	for i := range s.WorkerCount {
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

	createdTargetPod, err := kc.CoreV1().Pods(metav1.NamespaceDefault).Create(s.Context(), &corev1.Pod{
		TypeMeta:   metav1.TypeMeta{Kind: "Pod", APIVersion: "v1"},
		ObjectMeta: metav1.ObjectMeta{Name: "nginx-worker0"},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{{Name: "nginx-worker0", Image: "docker.io/library/nginx:1.31.1-alpine"}},
			NodeSelector: map[string]string{
				"kubernetes.io/hostname": "worker0",
			},
		},
	}, metav1.CreateOptions{})
	s.Require().NoError(err)
	s.Require().NoError(common.WaitForPod(s.Context(), kc, "nginx-worker0", metav1.NamespaceDefault), "nginx-worker0 pod did not start")

	targetPod, err := kc.CoreV1().Pods(createdTargetPod.Namespace).Get(s.Context(), createdTargetPod.Name, metav1.GetOptions{})
	s.Require().NoError(err)

	sourcePod, err := kc.CoreV1().Pods(metav1.NamespaceDefault).Create(s.Context(), &corev1.Pod{
		TypeMeta:   metav1.TypeMeta{Kind: "Pod", APIVersion: "v1"},
		ObjectMeta: metav1.ObjectMeta{Name: "nginx-worker1"},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{{Name: "alpine", Image: "docker.io/library/nginx:1.31.1-alpine"}},
			NodeSelector: map[string]string{
				"kubernetes.io/hostname": "worker1",
			},
		},
	}, metav1.CreateOptions{})
	s.Require().NoError(err)
	s.NoError(common.WaitForPod(s.Context(), kc, "nginx-worker1", metav1.NamespaceDefault), "nginx-worker1 pod did not start")

	// test both ipv4 and ipv6 addresses
	podIPs := map[string]string{}
	podIPs["ipv4"], podIPs["ipv6"] = s.getPodIPs(targetPod)
	for ipVersion, podIP := range podIPs {
		err := wait.PollUntilContextTimeout(s.Context(), 100*time.Millisecond, time.Minute, true, func(ctx context.Context) (done bool, err error) {
			target := net.JoinHostPort(podIP, "80")
			out, err := common.PodExecCmdOutput(kc, restConfig, sourcePod.Name, sourcePod.Namespace, "/usr/bin/wget -qO- http://"+target)
			s.T().Logf("Trying to access %s address %s: %s", ipVersion, target, out)
			if err != nil {
				s.T().Logf("error calling %s address: %v", ipVersion, err)
				return false, nil
			}
			s.T().Logf("server response from %s address: %s", ipVersion, out)
			return strings.Contains(out, "Welcome to nginx"), nil
		})
		s.Require().NoErrorf(err, "failed to access nginx server via %s address", ipVersion)
	}

	s.client = client

	s.T().Log("Validate the kube-dns service address")
	s.validateKubeDNSIP(client)

	s.T().Log("Verifying that pods didn't restart")
	// CoreDNS may not be ready when we finish the test, which can break VerifyNoRestartedPods,
	// so we need to wait for it first.
	s.NoError(common.WaitForCoreDNSReady(s.Context(), client))
	// Verify that there aren't containers restarted on kube-system
	for _, err := range common.VerifyNoRestartedPods(s.Context(), client) {
		s.NoError(err)
	}
}

func (s *DualstackSuite) getPodIPs(pod *corev1.Pod) (string, string) {
	s.Require().Len(pod.Status.PodIPs, 2)
	ipv4, ipv6 := pod.Status.PodIPs[0].IP, pod.Status.PodIPs[1].IP
	if s.defaultIPv6 {
		ipv4, ipv6 = pod.Status.PodIPs[1].IP, pod.Status.PodIPs[0].IP
	}
	s.Require().NotNil(net.ParseIP(ipv4).To4(), "pod has an unexpected non IPv4 address %q", ipv4)
	s.Require().Nil(net.ParseIP(ipv6).To4(), "pod has an unexpected IPv4 address %q", ipv6)
	s.Require().NotNil(net.ParseIP(ipv6).To16(), "pod has an unexpected non IPv6 address %q", ipv6)

	return ipv4, ipv6
}

func (s *DualstackSuite) validateKubeDNSIP(client *k8s.Clientset) {
	svc, err := client.CoreV1().Services(metav1.NamespaceSystem).Get(s.Context(), "kube-dns", metav1.GetOptions{})
	s.NoError(err, "failed to get service kube-dns")
	svcIP := net.ParseIP(svc.Spec.ClusterIP)
	if s.defaultIPv6 {
		s.Require().Nil(svcIP.To4(), "kube-dns has an unexpected IPv4 address")
		s.Require().NotNil(svcIP.To16(), "kube-dns has an invalid IP address")
	} else {
		s.Require().NotNil(svcIP.To4(), "kube-dns has an unexpected non IPv4 address")
	}
}

func (s *DualstackSuite) getIPv6Address(nodeName string) string {
	ssh, err := s.SSH(s.Context(), nodeName)
	s.Require().NoError(err)
	defer ssh.Disconnect()

	ipAddress, err := ssh.ExecWithOutput(s.Context(), "ip -6 -oneline addr show eth0 | awk '{print $4}' | grep -v '^fe80' | cut -d/ -f1")
	s.Require().NoError(err)
	return ipAddress

}

func TestDualStack(t *testing.T) {

	s := DualstackSuite{
		common.BootlooseSuite{
			ControllerCount: 1,
			WorkerCount:     2,
		},
		nil,
		false,
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
    serviceCIDR: 10.96.0.0/12
`

const k0sConfigWithKuberouterDualStack = `
spec:
  api:
    externalAddress: %s
  network:
    provider: kuberouter
    dualStack:
      enabled: true
      IPv6podCIDR: "fd00::/108"
      IPv6serviceCIDR: "fd01::/108"
    podCIDR: 10.244.0.0/16
    serviceCIDR: 10.96.0.0/12
`
