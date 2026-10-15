// SPDX-FileCopyrightText: 2021 k0s authors
// SPDX-License-Identifier: Apache-2.0

package calico

import (
	"context"
	"net"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/yaml"

	"github.com/k0sproject/k0s/inttest/common"
	"github.com/k0sproject/k0s/inttest/common/ociimages"
	"github.com/k0sproject/k0s/pkg/apis/k0s/v1beta1"
)

// The MTU of the nodes' network interface, below the usual 1500.
const hostMTU = 1400

type CalicoSuite struct {
	common.BootlooseSuite
	isIPv6Only bool
}

func (s *CalicoSuite) TestK0sGetsUp() {
	ctx := s.Context()

	{
		clusterCfg := &v1beta1.ClusterConfig{
			Spec: &v1beta1.ClusterSpec{
				Network: func() *v1beta1.Network {
					network := v1beta1.DefaultNetwork()
					network.Provider = "calico"
					network.Calico = &v1beta1.Calico{
						EnvVars: map[string]string{
							"TEST_BOOL_VAR":   "true",
							"TEST_INT_VAR":    "42",
							"TEST_STRING_VAR": "test",
						},
					}
					return network
				}(),
			},
		}
		if s.isIPv6Only {
			clusterCfg.Spec.Network.PodCIDR = "fd00::/108"
			clusterCfg.Spec.Network.ServiceCIDR = "fd01::/108"
		}

		config, err := yaml.Marshal(clusterCfg)
		s.Require().NoError(err)
		s.WriteFileContent(s.ControllerNode(0), "/tmp/k0s.yaml", config)
	}

	// Use a host MTU below 1500, so that a fixed overlay MTU (e.g. 1450) won't
	// fit into the tunnel. The MTU is left unset in the cluster config, so
	// Calico has to detect it on each node.
	nodes := []string{s.ControllerNode(0)}
	for i := range s.WorkerCount {
		nodes = append(nodes, s.WorkerNode(i))
	}
	for _, node := range nodes {
		ssh, err := s.SSH(ctx, node)
		s.Require().NoError(err)
		_, err = ssh.ExecWithOutput(ctx, "ip link set dev eth0 mtu "+strconv.Itoa(hostMTU))
		ssh.Disconnect()
		s.Require().NoError(err, "failed to set MTU on %s", node)
	}

	s.Require().NoError(s.InitController(0, "--config=/tmp/k0s.yaml", "--feature-gates=IPv6SingleStack=true"))

	if s.isIPv6Only {
		s.T().Log("Setting up IPv6 DNS for workers")
		common.ConfigureIPv6ResolvConf(&s.BootlooseSuite)
	}
	s.Require().NoError(s.RunWorkers())

	restConfig, err := s.GetKubeConfig(s.ControllerNode(0), "")
	s.NoError(err)
	kc, err := kubernetes.NewForConfig(restConfig)
	s.Require().NoError(err)

	err = s.WaitForNodeReady("worker0", kc)
	s.NoError(err)

	err = s.WaitForNodeReady("worker1", kc)
	s.NoError(err)

	calicoDaemonset, err := kc.AppsV1().DaemonSets(metav1.NamespaceSystem).Get(ctx, "calico-node", metav1.GetOptions{})
	s.Require().NoError(err)
	var calicoCustomEnvVarsFound int
	for _, v := range calicoDaemonset.Spec.Template.Spec.Containers[0].Env {
		if v.Name == "TEST_BOOL_VAR" || v.Name == "TEST_INT_VAR" || v.Name == "TEST_STRING_VAR" {
			calicoCustomEnvVarsFound++
		}
	}
	s.Equal(3, calicoCustomEnvVarsFound, "expecting to see custom calico env vars")

	s.AssertSomeKubeSystemPods(kc)

	s.T().Log("waiting to see calico pods ready")
	s.NoError(common.WaitForDaemonSet(ctx, kc, "calico-node", metav1.NamespaceSystem), "calico did not start")
	s.Require().NoError(common.VerifyKonnectivityMesh(ctx, restConfig, kc, s.T(), uint(s.ControllerCount), uint(s.WorkerCount)), "While verifying konnectivity mesh")

	createdTargetPod, err := kc.CoreV1().Pods(metav1.NamespaceDefault).Create(ctx, &corev1.Pod{
		TypeMeta:   metav1.TypeMeta{Kind: "Pod", APIVersion: "v1"},
		ObjectMeta: metav1.ObjectMeta{Name: "nginx"},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{{
				Name:  "nginx",
				Image: ociimages.Nginx,
				ReadinessProbe: &corev1.Probe{
					ProbeHandler: corev1.ProbeHandler{
						HTTPGet: &corev1.HTTPGetAction{
							Path:   "/",
							Port:   intstr.FromInt(80),
							Scheme: corev1.URISchemeHTTP,
						},
					},
				},
			}},
			NodeSelector: map[string]string{
				"kubernetes.io/hostname": "worker0",
			},
		},
	}, metav1.CreateOptions{})
	s.Require().NoError(err)
	s.Require().NoError(common.WaitForPod(ctx, kc, "nginx", metav1.NamespaceDefault), "nginx pod did not start")

	targetPod, err := kc.CoreV1().Pods(createdTargetPod.Namespace).Get(ctx, createdTargetPod.Name, metav1.GetOptions{})
	s.Require().NoError(err)

	alpineImage, err := ociimages.Alpine(ctx)
	s.Require().NoError(err)
	sourcePod, err := kc.CoreV1().Pods(metav1.NamespaceDefault).Create(ctx, &corev1.Pod{
		TypeMeta:   metav1.TypeMeta{Kind: "Pod", APIVersion: "v1"},
		ObjectMeta: metav1.ObjectMeta{Name: "alpine"},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{{
				Name:    "alpine",
				Image:   alpineImage,
				Command: []string{"sleep", "infinity"},
			}},
			NodeSelector: map[string]string{
				"kubernetes.io/hostname": "worker1",
			},
		},
	}, metav1.CreateOptions{})
	s.Require().NoError(err)
	s.NoError(common.WaitForPod(ctx, kc, "alpine", metav1.NamespaceDefault), "alpine pod did not start")

	s.Run("pod MTU matches the tunnel MTU", func() {
		tunnelDevice := "vxlan.calico"
		if s.isIPv6Only {
			tunnelDevice = "vxlan-v6.calico"
		}
		tunnelMTU, err := s.RunCommandWorker(1, "cat /sys/class/net/"+tunnelDevice+"/mtu")
		s.Require().NoError(err)
		podMTU, err := common.PodExecCmdOutput(kc, restConfig, sourcePod.Name, sourcePod.Namespace, "cat /sys/class/net/eth0/mtu")
		s.Require().NoError(err)
		podMTU = strings.TrimSpace(podMTU)

		s.T().Logf("Host MTU: %d, tunnel MTU: %s, pod MTU: %s", hostMTU, tunnelMTU, podMTU)
		s.Equal(tunnelMTU, podMTU, "pod MTU doesn't match the MTU of %s", tunnelDevice)
		if mtu, err := strconv.Atoi(podMTU); s.NoError(err) {
			s.Less(mtu, hostMTU, "pod MTU doesn't leave room for the VXLAN overhead")
		}
	})

	err = wait.PollImmediateWithContext(ctx, 100*time.Millisecond, time.Minute, func(ctx context.Context) (done bool, err error) {
		out, err := common.PodExecCmdOutput(kc, restConfig, sourcePod.Name, sourcePod.Namespace,
			"/usr/bin/wget -qO- "+net.JoinHostPort(targetPod.Status.PodIP, "80"))
		if err != nil {
			s.T().Log("Error calling nginx pod:", err)
			return false, nil
		}
		s.T().Log("server response", out)
		return strings.Contains(out, "Welcome to nginx"), nil
	})
	s.Require().NoError(err)

	// After the tests finished, verify that there are no restarted pods
	for _, err := range common.VerifyNoRestartedPods(ctx, kc) {
		s.NoError(err)
	}
}

func TestCalicoSuite(t *testing.T) {
	s := CalicoSuite{
		BootlooseSuite: common.BootlooseSuite{
			ControllerCount: 1,
			WorkerCount:     2,
		},
	}

	if strings.Contains(os.Getenv("K0S_INTTEST_TARGET"), "ipv6") {
		t.Log("Configuring IPv6 only networking")
		s.isIPv6Only = true
		s.Networks = []string{"bridge-ipv6"}
		s.AirgapImageBundleMountPoints = []string{"/var/lib/k0s/images/bundle.tar"}
		s.K0sExtraImageBundleMountPoints = []string{"/var/lib/k0s/images/ipv6.tar"}
	}
	suite.Run(t, &s)
}
