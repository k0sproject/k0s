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
package podsecurity

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/suite"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/utils/pointer"

	"github.com/k0sproject/k0s/inttest/common"
)

type PodSecuritySuite struct {
	common.FootlooseSuite
}

func (s *PodSecuritySuite) TestK0sGetsUp() {
	s.PutFile(s.ControllerNode(0), "/tmp/k0s.yaml", k0sConfigWithRestrictedPSS)
	s.NoError(s.InitController(0, "--config=/tmp/k0s.yaml", "--enable-worker", "--disable-components", "default-psp"))

	kc, err := s.KubeClient(s.ControllerNode(0))
	s.NoError(err)

	err = s.WaitForNodeReady(s.ControllerNode(0), kc)
	s.NoError(err)

	pods, err := kc.CoreV1().Pods("kube-system").List(context.TODO(), v1.ListOptions{
		Limit: 100,
	})
	s.NoError(err)

	podCount := len(pods.Items)

	s.T().Logf("found %d pods in kube-system", podCount)
	s.Greater(podCount, 0, "expecting to see few pods in kube-system namespace")

	ukc, err := s.UserKubeClient(s.ControllerNode(0))
	s.NoError(err)

	s.PutFile(s.ControllerNode(0), "/tmp/role.yaml", k0sTestUserRoleBinding)

	ssh, err := s.SSH(s.ControllerNode(0))
	s.NoError(err)
	defer ssh.Disconnect()

	_, err = ssh.ExecWithOutput(fmt.Sprintf("%s kubectl apply -f /tmp/role.yaml", s.K0sFullPath))
	s.NoError(err)

	s.T().Run("successfully_deploy_non-priveleged_pod", func(t *testing.T) {
		nonPrivelegedPodReq := &corev1.Pod{
			TypeMeta:   v1.TypeMeta{Kind: "Pod", APIVersion: "v1"},
			ObjectMeta: v1.ObjectMeta{Name: "test-pod-non-privileged"},
			Spec: corev1.PodSpec{
				Containers: []corev1.Container{{
					Name:  "pause",
					Image: "k8s.gcr.io/pause",
					SecurityContext: &corev1.SecurityContext{
						AllowPrivilegeEscalation: pointer.Bool(false),
						RunAsNonRoot:             pointer.Bool(true),
						Capabilities: &corev1.Capabilities{
							Drop: []corev1.Capability{"ALL"},
						},
						SeccompProfile: &corev1.SeccompProfile{
							Type: corev1.SeccompProfileTypeRuntimeDefault,
						},
					},
				}},
			},
		}

		_, err = ukc.CoreV1().Pods("default").Create(context.TODO(), nonPrivelegedPodReq, v1.CreateOptions{})
		s.NoError(err)
	})

	s.T().Run("returns_error_for_priveleged_pod", func(t *testing.T) {
		privelegedPodReq := &corev1.Pod{
			TypeMeta:   v1.TypeMeta{Kind: "Pod", APIVersion: "v1"},
			ObjectMeta: v1.ObjectMeta{Name: "test-pod-privileged"},
			Spec: corev1.PodSpec{
				Containers: []corev1.Container{
					{
						Name:  "pause",
						Image: "k8s.gcr.io/pause",
						SecurityContext: &corev1.SecurityContext{
							Privileged:   pointer.Bool(true),
							RunAsNonRoot: pointer.Bool(true),
							Capabilities: &corev1.Capabilities{
								Drop: []corev1.Capability{"ALL"},
							},
							SeccompProfile: &corev1.SeccompProfile{
								Type: corev1.SeccompProfileTypeRuntimeDefault,
							},
						},
					},
				},
			},
		}

		_, err = ukc.CoreV1().Pods("default").Create(context.TODO(), privelegedPodReq, v1.CreateOptions{})
		// Should return and error:
		// pods "test-pod-privileged" is forbidden: violates PodSecurity "restricted:latest": privileged (container "pause" must not set securityContext.privileged=true), allowPrivilegeEscalation != false (container "pause" must set securityContext.allowPrivilegeEscalation=false)
		s.Error(err)
	})

	s.T().Run("returns_error_for_run_as_root", func(t *testing.T) {
		privelegedPodReq := &corev1.Pod{
			TypeMeta:   v1.TypeMeta{Kind: "Pod", APIVersion: "v1"},
			ObjectMeta: v1.ObjectMeta{Name: "test-pod-privileged"},
			Spec: corev1.PodSpec{
				Containers: []corev1.Container{
					{
						Name:  "pause",
						Image: "k8s.gcr.io/pause",
						SecurityContext: &corev1.SecurityContext{
							RunAsUser: pointer.Int64(0),
							Capabilities: &corev1.Capabilities{
								Drop: []corev1.Capability{"ALL"},
							},
							SeccompProfile: &corev1.SeccompProfile{
								Type: corev1.SeccompProfileTypeRuntimeDefault,
							},
						},
					},
				},
			},
		}

		_, err = ukc.CoreV1().Pods("default").Create(context.TODO(), privelegedPodReq, v1.CreateOptions{})
		// Should return and error:
		// pods "test-pod-privileged" is forbidden: violates PodSecurity "restricted:latest": allowPrivilegeEscalation != false (container "pause" must set securityContext.allowPrivilegeEscalation=false), runAsNonRoot != true (pod or container "pause" must set securityContext.runAsNonRoot=true), runAsUser=0 (container "pause" must not set runAsUser=0)
		s.Error(err)
	})
}

func TestPodSecuritySuite(t *testing.T) {
	s := PodSecuritySuite{
		common.FootlooseSuite{
			ControllerCount: 1,
		},
	}
	suite.Run(t, &s)
}

const k0sConfigWithRestrictedPSS = `
spec:
  api:
    extraArgs:
      disable-admission-plugins: PodSecurityPolicy
  podSecurity:
    enforce: restricted
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
func (s *PodSecuritySuite) UserKubeClient(node string, k0sKubeconfigArgs ...string) (*kubernetes.Clientset, error) {
	cfg, err := s.CreateUserAndGetKubeClientConfig(node, "test", k0sKubeconfigArgs...)
	if err != nil {
		return nil, err
	}
	return kubernetes.NewForConfig(cfg)
}
