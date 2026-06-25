// SPDX-FileCopyrightText: 2023 k0s authors
// SPDX-License-Identifier: Apache-2.0

package customcidrs

import (
	"testing"

	"github.com/k0sproject/k0s/inttest/common"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/kubernetes"

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

	restConfig, err := s.GetKubeConfig(s.ControllerNode(0))
	s.Require().NoError(err)
	kc, err := kubernetes.NewForConfig(restConfig)
	s.Require().NoError(err)

	for i := range s.WorkerCount {
		err = s.WaitForNodeReady(s.WorkerNode(i), kc)
		s.Require().NoError(err)
	}

	s.AssertSomeKubeSystemPods(kc)

	ctx := s.Context()

	s.Require().NoError(common.WaitForKubeRouterReady(ctx, kc))
	s.Require().NoError(common.WaitForCoreDNSReady(ctx, kc))

	s.T().Log("creating nginx pod to verify DNS settings")
	_, err = kc.CoreV1().Pods(metav1.NamespaceDefault).Create(s.Context(), &corev1.Pod{
		TypeMeta:   metav1.TypeMeta{Kind: "Pod", APIVersion: "v1"},
		ObjectMeta: metav1.ObjectMeta{Name: "nginx", Namespace: metav1.NamespaceDefault},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{{
				Name:  "nginx",
				Image: "docker.io/library/nginx:1.31.2-alpine",
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
				"kubernetes.io/hostname": s.WorkerNode(0),
			},
		},
	}, metav1.CreateOptions{})
	s.Require().NoError(err)
	s.Require().NoError(common.WaitForPod(ctx, kc, "nginx", metav1.NamespaceDefault))
	s.Require().NoError(common.VerifyKonnectivityMesh(ctx, restConfig, kc, s.T(), uint(s.ControllerCount), uint(s.WorkerCount)), "While verifying konnectivity mesh")

	// Check the pod resolv.conf is correct
	resolv, err := common.PodExecCmdOutput(kc, restConfig, "nginx", metav1.NamespaceDefault, "cat /etc/resolv.conf")
	s.NoError(err)
	s.Contains(resolv, "10.152.184.10")

	// Verify lookup actually works
	nslookup, err := common.PodExecCmdOutput(kc, restConfig, "nginx", metav1.NamespaceDefault, "nslookup kubernetes.default.svc.cluster.local")
	s.Require().NoError(err)
	s.Contains(nslookup, "Address: 10.152.184.1")

	// Check that we can access the kubernetes svc via DNS name
	kubeSvcOutput, err := common.PodExecCmdOutput(kc, restConfig, "nginx", metav1.NamespaceDefault, `curl -v -k --connect-timeout 2 -s -I https://kubernetes.default.svc.cluster.local`)
	s.Require().NoError(err)
	s.Contains(kubeSvcOutput, "HTTP/2 401")

	// Check that kube-router has the right service CIDR set
	ds, err := kc.AppsV1().DaemonSets(metav1.NamespaceSystem).Get(ctx, "kube-router", metav1.GetOptions{})
	s.Require().NoError(err)
	s.Contains(ds.Spec.Template.Spec.Containers[0].Args, "--service-cluster-ip-range=10.152.184.0/24")
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
