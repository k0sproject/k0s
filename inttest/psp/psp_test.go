/*
Copyright 2022 k0s authors

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

package psp

import (
	"fmt"
	"testing"

	"github.com/k0sproject/k0s/inttest/common"

	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/utils/ptr"

	"github.com/stretchr/testify/suite"
)

type PSPSuite struct {
	common.BootlooseSuite
}

func (s *PSPSuite) TestK0sGetsUp() {
	s.PutFile(s.ControllerNode(0), "/tmp/k0s.yaml", k0sConfigWithRestrictedPSP)
	s.NoError(s.InitController(0, "--config=/tmp/k0s.yaml", "--enable-worker"))

	kc, err := s.KubeClient(s.ControllerNode(0))
	s.Require().NoError(err)

	err = s.WaitForNodeReady(s.ControllerNode(0), kc)
	s.NoError(err)

	s.AssertSomeKubeSystemPods(kc)

	s.PutFile(s.ControllerNode(0), "/tmp/role.yaml", k0sTestUserRoleBinding)

	ssh, err := s.SSH(s.Context(), s.ControllerNode(0))
	s.Require().NoError(err)
	defer ssh.Disconnect()

	_, err = ssh.ExecWithOutput(s.Context(), fmt.Sprintf("%s kubectl apply -f /tmp/role.yaml", s.K0sFullPath))
	s.NoError(err)

	nonPrivelegedPodReq := &corev1.Pod{
		TypeMeta:   v1.TypeMeta{Kind: "Pod", APIVersion: "v1"},
		ObjectMeta: v1.ObjectMeta{Name: "test-pod-non-privileged"},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{{Name: "pause", Image: "registry.k8s.io/pause"}},
		},
	}

	ukc, err := s.UserKubeClient(s.ControllerNode(0))
	s.Require().NoError(err)

	_, err = ukc.CoreV1().Pods("default").Create(s.Context(), nonPrivelegedPodReq, v1.CreateOptions{})
	s.NoError(err)

	privelegedPodReq := &corev1.Pod{
		TypeMeta:   v1.TypeMeta{Kind: "Pod", APIVersion: "v1"},
		ObjectMeta: v1.ObjectMeta{Name: "test-pod-privileged"},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:  "pause",
					Image: "registry.k8s.io/pause",
					SecurityContext: &corev1.SecurityContext{
						RunAsUser: ptr.To(int64(0)),
					},
				},
			},
		},
	}

	_, err = ukc.CoreV1().Pods("default").Create(s.Context(), privelegedPodReq, v1.CreateOptions{})
	s.NoError(err)
}

func TestPSPSuite(t *testing.T) {
	s := PSPSuite{
		common.BootlooseSuite{
			ControllerCount: 1,
		},
	}
	suite.Run(t, &s)
}

const k0sConfigWithRestrictedPSP = `
spec:
  podSecurityPolicy:
    defaultPolicy: 99-k0s-restricted
`

const k0sTestUserRoleBinding = `
kind: RoleBinding
apiVersion: rbac.authorization.k8s.io/v1
metadata:
  name: test-rolebinding
  namespace: default
roleRef:
  kind: ClusterRole
  name: edit 
  apiGroup: rbac.authorization.k8s.io
subjects:
- kind: User
  name: test
  apiGroup: rbac.authorization.k8s.io
`

// KubeClient return kube client by loading the admin access config from given node
func (s *PSPSuite) UserKubeClient(node string, k0sKubeconfigArgs ...string) (*kubernetes.Clientset, error) {
	cfg, err := s.CreateUserAndGetKubeClientConfig(node, "test", k0sKubeconfigArgs...)
	if err != nil {
		return nil, err
	}
	return kubernetes.NewForConfig(cfg)
}
