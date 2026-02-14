// SPDX-FileCopyrightText: 2021 k0s authors
// SPDX-License-Identifier: Apache-2.0

package calico

import (
	"context"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"sigs.k8s.io/yaml"

	"github.com/k0sproject/k0s/inttest/common"
	"github.com/k0sproject/k0s/pkg/apis/k0s/v1beta1"
)

type CalicoSuite struct {
	common.BootlooseSuite
	isIPv6Only bool
}

func (s *CalicoSuite) TestK0sGetsUp() {

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

	s.Require().NoError(s.InitController(0, "--config=/tmp/k0s.yaml", "--feature-gates=IPv6SingleStack=true"))

	if s.isIPv6Only {
		s.T().Log("Setting up IPv6 DNS for workers")
		common.ConfigureIPv6ResolvConf(&s.BootlooseSuite)
	}
	s.Require().NoError(s.RunWorkers())

	kc, err := s.KubeClient("controller0", "")
	s.Require().NoError(err)
	restConfig, err := s.GetKubeConfig("controller0", "")
	s.NoError(err)

	err = s.WaitForNodeReady("worker0", kc)
	s.NoError(err)

	err = s.WaitForNodeReady("worker1", kc)
	s.NoError(err)

	calicoDaemonset, err := kc.AppsV1().DaemonSets(metav1.NamespaceSystem).Get(context.TODO(), "calico-node", metav1.GetOptions{})
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
	s.NoError(common.WaitForDaemonSet(s.Context(), kc, "calico-node", metav1.NamespaceSystem), "calico did not start")
	s.NoError(common.WaitForPodLogs(s.Context(), kc, metav1.NamespaceSystem))

	createdTargetPod, err := kc.CoreV1().Pods(metav1.NamespaceDefault).Create(s.Context(), &corev1.Pod{
		TypeMeta:   metav1.TypeMeta{Kind: "Pod", APIVersion: "v1"},
		ObjectMeta: metav1.ObjectMeta{Name: "nginx"},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{{Name: "nginx", Image: "docker.io/library/nginx:1.29.5-alpine"}},
			NodeSelector: map[string]string{
				"kubernetes.io/hostname": "worker0",
			},
		},
	}, metav1.CreateOptions{})
	s.Require().NoError(err)
	s.Require().NoError(common.WaitForPod(s.Context(), kc, "nginx", metav1.NamespaceDefault), "nginx pod did not start")

	targetPod, err := kc.CoreV1().Pods(createdTargetPod.Namespace).Get(s.Context(), createdTargetPod.Name, metav1.GetOptions{})
	s.Require().NoError(err)

	sourcePod, err := kc.CoreV1().Pods(metav1.NamespaceDefault).Create(s.Context(), &corev1.Pod{
		TypeMeta:   metav1.TypeMeta{Kind: "Pod", APIVersion: "v1"},
		ObjectMeta: metav1.ObjectMeta{Name: "alpine"},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{{
				Name:    "alpine",
				Image:   "docker.io/library/alpine:" + getAlpineVersion(s.T()),
				Command: []string{"sleep", "infinity"},
			}},
			NodeSelector: map[string]string{
				"kubernetes.io/hostname": "worker1",
			},
		},
	}, metav1.CreateOptions{})
	s.Require().NoError(err)
	s.NoError(common.WaitForPod(s.Context(), kc, "alpine", metav1.NamespaceDefault), "alpine pod did not start")

	err = wait.PollImmediateWithContext(s.Context(), 100*time.Millisecond, time.Minute, func(ctx context.Context) (done bool, err error) {
		out, err := common.PodExecCmdOutput(kc, restConfig, sourcePod.Name, sourcePod.Namespace,
			"/usr/bin/wget -qO- "+net.JoinHostPort(targetPod.Status.PodIP, "80"))
		if err != nil {
			return false, err
		}
		s.T().Log("server response", out)
		return strings.Contains(out, "Welcome to nginx"), nil
	})
	s.Require().NoError(err)
}

func getAlpineVersion(t *testing.T) string {
	cmd := exec.Command("."+string(filepath.Separator)+"vars.sh", "alpine_version")
	cmd.Dir = filepath.Join("..", "..")
	out, err := cmd.Output()
	require.NoError(t, err)
	version, _, _ := strings.Cut(string(out), "\n")
	require.NotEmpty(t, version, "Failed to get Alpine version")
	return version
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
