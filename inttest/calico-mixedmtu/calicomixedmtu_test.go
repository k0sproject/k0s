// SPDX-FileCopyrightText: 2026 k0s authors
// SPDX-License-Identifier: Apache-2.0

package calicomixedmtu

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/yaml"

	"github.com/k0sproject/k0s/inttest/common"
	"github.com/k0sproject/k0s/inttest/common/ociimages"
	"github.com/k0sproject/k0s/pkg/apis/k0s/v1beta1"
)

// The MTU of each node's network interface.
var nodeMTUs = map[string]int{
	"controller0": 1500,
	"worker0":     1450,
	"worker1":     1400,
}

// The smallest MTU of any node, minus the VXLAN overhead, which is what the
// docs tell users to set when nodes have different MTUs.
const overlayMTU = 1400 - 50

type CalicoMixedMTUSuite struct {
	common.BootlooseSuite
}

// Verifies that pod traffic works between nodes with different MTUs when the
// Calico MTU is set to fit the node with the smallest MTU. Neither the 1450
// default nor auto-detection (0) would work here: pods on the 1500 MTU node
// would send encapsulated packets that the 1400 MTU node drops.
func (s *CalicoMixedMTUSuite) TestPodTrafficBetweenNodesWithDifferentMTUs() {
	ctx := s.Context()

	network := v1beta1.DefaultNetwork()
	network.Provider = "calico"
	network.Calico = v1beta1.DefaultCalico()
	network.Calico.MTU = overlayMTU
	config, err := yaml.Marshal(&v1beta1.ClusterConfig{Spec: &v1beta1.ClusterSpec{Network: network}})
	s.Require().NoError(err)
	s.WriteFileContent(s.ControllerNode(0), "/tmp/k0s.yaml", config)

	for node, mtu := range nodeMTUs {
		s.Require().NoError(s.setMTU(ctx, node, mtu))
	}

	s.Require().NoError(s.InitController(0, "--config=/tmp/k0s.yaml", "--enable-worker", "--no-taints"))
	s.Require().NoError(s.RunWorkers())

	restConfig, err := s.GetKubeConfig(s.ControllerNode(0))
	s.Require().NoError(err)
	kc, err := kubernetes.NewForConfig(restConfig)
	s.Require().NoError(err)
	for node := range nodeMTUs {
		s.Require().NoError(s.WaitForNodeReady(node, kc))
	}
	s.Require().NoError(common.WaitForDaemonSet(ctx, kc, "calico-node", metav1.NamespaceSystem), "calico did not start")

	s.T().Log("Starting a pod on each node")
	pods := s.startPodOnEachNode(ctx, kc)

	s.Run("every node uses the configured overlay MTU", func() {
		for node, pod := range pods {
			tunnelMTU, err := s.readFile(ctx, node, "/sys/class/net/vxlan.calico/mtu")
			s.Require().NoError(err)
			podMTU, err := common.PodExecCmdOutput(kc, restConfig, pod.Name, pod.Namespace, "cat /sys/class/net/eth0/mtu")
			s.Require().NoError(err)
			podMTU = strings.TrimSpace(podMTU)

			s.T().Logf("%s: host MTU %d, tunnel MTU %s, pod MTU %s", node, nodeMTUs[node], tunnelMTU, podMTU)
			s.Equal(strconv.Itoa(overlayMTU), tunnelMTU, "unexpected MTU of vxlan.calico on %s", node)
			s.Equal(strconv.Itoa(overlayMTU), podMTU, "unexpected MTU of pod %s on %s", pod.Name, node)
		}
	})

	// Use ICMP, so that nothing like the TCP maximum segment size can hide
	// packets that are too large for a node's network interface.
	for from, fromPod := range pods {
		for to, toPod := range pods {
			if from == to {
				continue
			}
			s.Run(fmt.Sprintf("ping from %s to %s", from, to), func() {
				ping := func(size int) error {
					_, err := common.PodExecCmdOutput(kc, restConfig, fromPod.Name, fromPod.Namespace,
						fmt.Sprintf("ping -c 3 -W 2 -s %d %s", size, toPod.Status.PodIP))
					return err
				}

				// Wait until the pods can reach each other at all.
				s.Require().NoError(wait.PollUntilContextTimeout(ctx, time.Second, time.Minute, true, func(context.Context) (bool, error) {
					return ping(56) == nil, nil
				}))

				for _, size := range []int{1400, 3000, 8000} {
					s.NoError(ping(size), "%d bytes from %s to %s didn't get through", size, from, to)
				}
			})
		}
	}
}

func (s *CalicoMixedMTUSuite) setMTU(ctx context.Context, node string, mtu int) error {
	ssh, err := s.SSH(ctx, node)
	if err != nil {
		return err
	}
	defer ssh.Disconnect()
	_, err = ssh.ExecWithOutput(ctx, "ip link set dev eth0 mtu "+strconv.Itoa(mtu))
	return err
}

func (s *CalicoMixedMTUSuite) readFile(ctx context.Context, node, path string) (string, error) {
	ssh, err := s.SSH(ctx, node)
	if err != nil {
		return "", err
	}
	defer ssh.Disconnect()
	return ssh.ExecWithOutput(ctx, "cat "+path)
}

// Starts a pod on each node and returns them, indexed by node name.
func (s *CalicoMixedMTUSuite) startPodOnEachNode(ctx context.Context, kc kubernetes.Interface) map[string]*corev1.Pod {
	alpineImage, err := ociimages.Alpine(ctx)
	s.Require().NoError(err)

	labels := map[string]string{"app": "mtu-test"}
	_, err = kc.AppsV1().DaemonSets(metav1.NamespaceDefault).Create(ctx, &appsv1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{Name: "mtu-test"},
		Spec: appsv1.DaemonSetSpec{
			Selector: &metav1.LabelSelector{MatchLabels: labels},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: labels},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{{
						Name:    "alpine",
						Image:   alpineImage,
						Command: []string{"sleep", "infinity"},
					}},
				},
			},
		},
	}, metav1.CreateOptions{})
	s.Require().NoError(err)

	// Wait for a pod on every node. A new DaemonSet has no pods scheduled
	// yet, so its available and desired counts start out equal at zero.
	s.Require().NoError(wait.PollUntilContextTimeout(ctx, time.Second, 5*time.Minute, true, func(ctx context.Context) (bool, error) {
		ds, err := kc.AppsV1().DaemonSets(metav1.NamespaceDefault).Get(ctx, "mtu-test", metav1.GetOptions{})
		if err != nil {
			return false, nil
		}
		return ds.Status.DesiredNumberScheduled == int32(len(nodeMTUs)) &&
			ds.Status.NumberAvailable == ds.Status.DesiredNumberScheduled, nil
	}), "mtu-test pods did not start")

	podList, err := kc.CoreV1().Pods(metav1.NamespaceDefault).List(ctx, metav1.ListOptions{LabelSelector: "app=mtu-test"})
	s.Require().NoError(err)
	pods := make(map[string]*corev1.Pod, len(podList.Items))
	for i := range podList.Items {
		pods[podList.Items[i].Spec.NodeName] = &podList.Items[i]
	}
	s.Require().Len(pods, len(nodeMTUs), "expected one pod per node")
	return pods
}

func TestCalicoMixedMTUSuite(t *testing.T) {
	suite.Run(t, &CalicoMixedMTUSuite{
		BootlooseSuite: common.BootlooseSuite{
			ControllerCount: 1,
			WorkerCount:     2,
		},
	})
}
