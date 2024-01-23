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

package calico

import (
	"context"
	"fmt"
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

	"github.com/k0sproject/k0s/inttest/common"
)

type CalicoSuite struct {
	common.BootlooseSuite
}

func (s *CalicoSuite) TestK0sGetsUp() {
	s.PutFile(s.ControllerNode(0), "/tmp/k0s.yaml", k0sConfig)
	s.Require().NoError(s.InitController(0, "--config=/tmp/k0s.yaml"))
	s.Require().NoError(s.RunWorkers())

	kc, err := s.KubeClient("controller0", "")
	s.Require().NoError(err)
	restConfig, err := s.GetKubeConfig("controller0", "")
	s.NoError(err)

	err = s.WaitForNodeReady("worker0", kc)
	s.NoError(err)

	err = s.WaitForNodeReady("worker1", kc)
	s.NoError(err)

	calicoDaemonset, err := kc.AppsV1().DaemonSets("kube-system").Get(context.TODO(), "calico-node", metav1.GetOptions{})
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
	s.NoError(common.WaitForDaemonSet(s.Context(), kc, "calico-node", "kube-system"), "calico did not start")
	s.NoError(common.WaitForPodLogs(s.Context(), kc, "kube-system"))

	createdTargetPod, err := kc.CoreV1().Pods("default").Create(s.Context(), &corev1.Pod{
		TypeMeta:   metav1.TypeMeta{Kind: "Pod", APIVersion: "v1"},
		ObjectMeta: metav1.ObjectMeta{Name: "nginx"},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{{Name: "nginx", Image: "docker.io/library/nginx:1.23.1-alpine"}},
			NodeSelector: map[string]string{
				"kubernetes.io/hostname": "worker0",
			},
		},
	}, metav1.CreateOptions{})
	s.Require().NoError(err)
	s.Require().NoError(common.WaitForPod(s.Context(), kc, "nginx", "default"), "nginx pod did not start")

	targetPod, err := kc.CoreV1().Pods(createdTargetPod.Namespace).Get(s.Context(), createdTargetPod.Name, metav1.GetOptions{})
	s.Require().NoError(err)

	sourcePod, err := kc.CoreV1().Pods("default").Create(s.Context(), &corev1.Pod{
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
	s.NoError(common.WaitForPod(s.Context(), kc, "alpine", "default"), "alpine pod did not start")

	err = wait.PollImmediateWithContext(s.Context(), 100*time.Millisecond, time.Minute, func(ctx context.Context) (done bool, err error) {
		out, err := common.PodExecCmdOutput(kc, restConfig, sourcePod.Name, sourcePod.Namespace, fmt.Sprintf("/usr/bin/wget -qO- %s", targetPod.Status.PodIP))
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
		common.BootlooseSuite{
			ControllerCount: 1,
			WorkerCount:     2,
		},
	}
	suite.Run(t, &s)
}

const k0sConfig = `
spec:
  network:
    provider: calico
    calico:
      envVars:
        TEST_BOOL_VAR: "true"
        TEST_INT_VAR: "42"
        TEST_STRING_VAR: test
`
