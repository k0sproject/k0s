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
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/k0sproject/k0s/inttest/common"
	"github.com/k0sproject/k0s/pkg/applier"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	kubeproxyv1alpha1 "k8s.io/kube-proxy/config/v1alpha1"

	"github.com/stretchr/testify/suite"
)

type DualstackSuite struct {
	common.BootlooseSuite

	client        *kubernetes.Clientset
	cni           string
	defaultIPv6   bool
	dynamicConfig bool
}

func (s *DualstackSuite) TestDualStackNodesHavePodCIDRs() {
	nl, err := s.client.CoreV1().Nodes().List(s.Context(), metav1.ListOptions{})
	s.Require().NoError(err)
	for _, n := range nl.Items {
		s.Require().Len(n.Spec.PodCIDRs, 2, "Each node must have ipv4 and ipv6 pod cidr")
	}
}

func (s *DualstackSuite) TestComponentsHaveCorrectCIDRs() {
	ctx := s.Context()
	expectedPodCIDRs := "10.233.0.0/16,fd00::/108"
	expectedServiceCIDRs := "10.112.0.0/12,fd01::/108"
	if s.defaultIPv6 {
		expectedPodCIDRs = "fd00::/108,10.233.0.0/16"
		expectedServiceCIDRs = "fd01::/108,10.112.0.0/12"
	}

	node := s.ControllerNode(0)

	ssh, err := s.SSH(ctx, node)
	if !s.NoError(err) {
		return
	}
	defer ssh.Disconnect()

	s.Run("kube-apiserver", func() {
		if args, err := cmdlineForExecutable(ctx, ssh, "kube-apiserver"); s.NoError(err) {
			s.Contains(args, "--service-cluster-ip-range="+expectedServiceCIDRs)
		}
	})

	s.Run("kube-controller-manager", func() {
		if args, err := cmdlineForExecutable(ctx, ssh, "kube-controller-manager"); s.NoError(err) {
			s.Contains(args, "--service-cluster-ip-range="+expectedServiceCIDRs)
			s.Contains(args, "--cluster-cidr="+expectedPodCIDRs)
		}
	})

	kc, err := s.KubeClient(node)
	if !s.NoError(err) {
		return
	}

	s.Run("kube-proxy", func() {
		cm, err := kc.CoreV1().ConfigMaps(metav1.NamespaceSystem).Get(s.Context(), "kube-proxy", metav1.GetOptions{})
		if !s.NoError(err) {
			return
		}

		var config kubeproxyv1alpha1.KubeProxyConfiguration
		codec := applier.CodecFor(applier.BuildScheme(kubeproxyv1alpha1.AddToScheme))
		obj, gvk, err := codec.Decode([]byte(cm.Data["config.conf"]), nil, &config)
		if !s.NoError(err) {
			return
		}

		if config, ok := obj.(*kubeproxyv1alpha1.KubeProxyConfiguration); s.Truef(ok, "Unexpected type: %s", gvk) {
			s.Equal(expectedPodCIDRs, config.ClusterCIDR)
		}
	})

	s.Run("kube-router", func() {
		if s.cni != "kuberouter" {
			s.T().Skip("Using", s.cni)
		}

		kc, err := s.KubeClient(node)
		if !s.NoError(err) {
			return
		}

		ds, err := kc.AppsV1().DaemonSets(metav1.NamespaceSystem).Get(s.Context(), "kube-router", metav1.GetOptions{})
		if !s.NoError(err) {
			return
		}

		s.Contains(ds.Spec.Template.Spec.Containers[0].Args, "--service-cluster-ip-range="+expectedServiceCIDRs)
	})
}

func cmdlineForExecutable(ctx context.Context, ssh *common.SSHConnection, binary string) ([]string, error) {
	output, err := ssh.ExecWithOutput(ctx, fmt.Sprintf("pidof -- %q", binary))
	if err != nil {
		return nil, err
	}

	pid, _, multiplePids := strings.Cut(output, " ")
	if multiplePids {
		return nil, errors.New("expected a single PID: " + output)
	}

	output, err = ssh.ExecWithOutput(ctx, fmt.Sprintf("cat /proc/%q/cmdline", pid))
	if err != nil {
		return nil, errors.New("failed to get cmdline for PID " + output)
	}

	return strings.Split(output, "\x00"), nil
}

func (s *DualstackSuite) SetupSuite() {
	isDockerIPv6Enabled, err := s.IsDockerIPv6Enabled()
	s.NoError(err)
	s.Require().True(isDockerIPv6Enabled, "Please enable IPv6 in docker before running this test")
	s.BootlooseSuite.SetupSuite()

	k0sConfig := k0sConfigWithCalicoDualStack
	if s.cni == "kuberouter" {
		s.T().Log("Using kube-router network")
		var ipAddress string
		if s.defaultIPv6 {
			ipAddress = s.getIPv6Address(s.ControllerNode(0))
		} else {
			ipAddress = s.GetIPAddress(s.ControllerNode(0))
		}
		k0sConfig = fmt.Sprintf(k0sConfigWithKuberouterDualStack, ipAddress)
	}
	s.PutFile(s.ControllerNode(0), "/tmp/k0s.yaml", k0sConfig)
	controllerArgs := []string{"--config=/tmp/k0s.yaml"}
	if s.dynamicConfig {
		s.T().Log("Enabling dynamic config for controller")
		controllerArgs = append(controllerArgs, "--enable-dynamic-config")
	}
	s.Require().NoError(s.InitController(0, controllerArgs...))
	s.Require().NoError(s.RunWorkers())
	client, err := s.KubeClient(s.ControllerNode(0))
	s.Require().NoError(err)
	for i := range s.WorkerCount {
		err = s.WaitForNodeReady(s.WorkerNode(i), client)
		s.Require().NoError(err)
		ssh, err := s.SSH(s.Context(), s.WorkerNode(i))
		s.Require().NoError(err)
		defer ssh.Disconnect()
		output, err := ssh.ExecWithOutput(s.Context(), "cat /proc/sys/net/ipv6/conf/all/disable_ipv6")
		s.Require().NoError(err)
		s.T().Logf("worker%d: /proc/sys/net/ipv6/conf/all/disable_ipv6=%s", i, output)
	}

	kc, err := s.KubeClient(s.ControllerNode(0), "")
	s.Require().NoError(err)
	restConfig, err := s.GetKubeConfig(s.ControllerNode(0), "")
	s.Require().NoError(err)

	targetPod, err := kc.CoreV1().Pods(metav1.NamespaceDefault).Create(s.Context(), &corev1.Pod{
		TypeMeta:   metav1.TypeMeta{Kind: "Pod", APIVersion: "v1"},
		ObjectMeta: metav1.ObjectMeta{Name: "nginx-" + s.WorkerNode(0)},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{{Name: "nginx", Image: "docker.io/library/nginx:1.31.1-alpine"}},
			NodeSelector: map[string]string{
				"kubernetes.io/hostname": s.WorkerNode(0),
			},
		},
	}, metav1.CreateOptions{})
	s.Require().NoError(err)

	sourcePod, err := kc.CoreV1().Pods(metav1.NamespaceDefault).Create(s.Context(), &corev1.Pod{
		TypeMeta:   metav1.TypeMeta{Kind: "Pod", APIVersion: "v1"},
		ObjectMeta: metav1.ObjectMeta{Name: "nginx-" + s.WorkerNode(1)},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{{Name: "alpine", Image: "docker.io/library/nginx:1.31.1-alpine"}},
			NodeSelector: map[string]string{
				"kubernetes.io/hostname": s.WorkerNode(1),
			},
		},
	}, metav1.CreateOptions{})
	s.Require().NoError(err)

	s.T().Logf("Waiting for pod: %s", targetPod.Name)
	s.Require().NoErrorf(common.WaitForPod(s.Context(), kc, targetPod.Name, metav1.NamespaceDefault), "%s pod did not start", targetPod.Name)
	targetPod, err = kc.CoreV1().Pods(targetPod.Namespace).Get(s.Context(), targetPod.Name, metav1.GetOptions{})
	s.Require().NoError(err)

	s.T().Logf("Waiting for pod: %s", sourcePod.Name)
	s.NoErrorf(common.WaitForPod(s.Context(), kc, sourcePod.Name, metav1.NamespaceDefault), "%s pod did not start", sourcePod.Name)

	// test both ipv4 and ipv6 addresses
	podIPs := map[string]string{}
	podIPs["ipv4"], podIPs["ipv6"] = s.getPodIPs(targetPod)
	for ipVersion, podIP := range podIPs {
		target := net.JoinHostPort(podIP, "80")
		s.T().Logf("Trying to access %s address %s of pod %s from pod %s", ipVersion, target, targetPod.Name, sourcePod.Name)
		err := wait.PollUntilContextTimeout(s.Context(), 100*time.Millisecond, time.Minute, true, func(ctx context.Context) (done bool, err error) {
			out, err := common.PodExecCmdOutput(kc, restConfig, sourcePod.Name, sourcePod.Namespace, "/usr/bin/wget -qO- http://"+target)
			if err != nil {
				s.T().Logf("Error calling %s address of pod %s: %v", ipVersion, targetPod.Name, err)
				return false, nil
			}
			if !strings.Contains(out, "Welcome to nginx") {
				s.T().Logf("Server response from %s address of pod %s: %s", ipVersion, targetPod.Name, out)
				return false, nil
			}

			s.T().Logf("Connection to %s address of pod %s from pod %s was successful", ipVersion, targetPod.Name, sourcePod.Name)
			return true, nil
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

func (s *DualstackSuite) validateKubeDNSIP(client *kubernetes.Clientset) {
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
	target := os.Getenv("K0S_INTTEST_TARGET")
	s := DualstackSuite{
		BootlooseSuite: common.BootlooseSuite{
			ControllerCount: 1,
			WorkerCount:     2,
		},
		cni:           "calico",
		dynamicConfig: strings.Contains(target, "dynamicconfig"),
	}

	if strings.Contains(target, "kuberouter") {
		s.cni = "kuberouter"
		s.defaultIPv6 = true
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
    podCIDR: 10.233.0.0/16
    serviceCIDR: 10.112.0.0/12
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
    podCIDR: 10.233.0.0/16
    serviceCIDR: 10.112.0.0/12
`
