/*
Copyright 2023 k0s authors

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

package customcidrs

import (
	"testing"

	"github.com/k0sproject/k0s/inttest/common"
	"k8s.io/apimachinery/pkg/util/intstr"

	"github.com/stretchr/testify/suite"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type CustomCIDRsSuite struct {
	common.BootlooseSuite
}

const k0sConfig = `
spec:
  storage:
    type: kine
  network:
    serviceCIDR: 10.152.184.0/24
    podCIDR: 10.3.0.0/16
`

func (s *CustomCIDRsSuite) TestK0sGetsUp() {
	s.PutFile(s.ControllerNode(0), "/tmp/k0s.yaml", k0sConfig)
	// Metrics disabled as it's super slow to get up properly and interferes with API discovery etc. while it's getting up
	s.Require().NoError(s.InitController(0, "--config=/tmp/k0s.yaml", "--disable-components metrics-server", "--enable-dynamic-config"))
	s.Require().NoError(s.RunWorkers())

	kc, err := s.KubeClient(s.ControllerNode(0))
	if err != nil {
		s.FailNow("failed to obtain Kubernetes client", err)
	}

	err = s.WaitForNodeReady(s.WorkerNode(0), kc)
	s.Require().NoError(err)

	err = s.WaitForNodeReady(s.WorkerNode(1), kc)
	s.Require().NoError(err)

	s.AssertSomeKubeSystemPods(kc)

	ctx := s.Context()

	s.Require().NoError(common.WaitForKubeRouterReady(ctx, kc))
	s.Require().NoError(common.WaitForCoreDNSReady(ctx, kc))

	s.T().Log("creating nginx pod to verify DNS settings")
	_, err = kc.CoreV1().Pods("default").Create(s.Context(), &corev1.Pod{
		TypeMeta:   metav1.TypeMeta{Kind: "Pod", APIVersion: "v1"},
		ObjectMeta: metav1.ObjectMeta{Name: "nginx", Namespace: "default"},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{{
				Name:  "nginx",
				Image: "docker.io/library/nginx:1.23.1-alpine",
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
	// Wait till we see the pod ready and are able to get logs
	// Getting logs means konnectivity tunnels are up and running
	s.Require().NoError(common.WaitForPod(ctx, kc, "nginx", "default"))
	s.Require().NoError(common.WaitForPodLogs(ctx, kc, "default"))

	restConfig, err := s.GetKubeConfig("controller0", "")
	s.Require().NoError(err)
	s.Require().NotNil(restConfig)

	// Check the pod resolv.conf is correct
	resolv, err := common.PodExecCmdOutput(kc, restConfig, "nginx", "default", "cat /etc/resolv.conf")
	s.NoError(err)
	s.Contains(resolv, "10.152.184.10")

	// Verify lookup actually works
	nslookup, err := common.PodExecCmdOutput(kc, restConfig, "nginx", "default", "nslookup kubernetes.default.svc.cluster.local")
	s.Require().NoError(err)
	s.Require().Contains(nslookup, "Address: 10.152.184.1")

	// Check that we can access the kubernetes svc via DNS name
	kubeSvcOutput, err := common.PodExecCmdOutput(kc, restConfig, "nginx", "default", `curl -v -k --connect-timeout 2 -s -I https://kubernetes.default.svc.cluster.local`)
	s.Require().NoError(err)

	s.Require().Contains(kubeSvcOutput, "HTTP/2 401")
}

func TestCustomCIDRsSuite(t *testing.T) {
	s := CustomCIDRsSuite{
		common.BootlooseSuite{
			ControllerCount: 1,
			WorkerCount:     2,
		},
	}
	suite.Run(t, &s)
}
