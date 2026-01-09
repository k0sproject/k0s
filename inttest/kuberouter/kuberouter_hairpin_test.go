// SPDX-FileCopyrightText: 2022 k0s authors
// SPDX-License-Identifier: Apache-2.0

package kuberouter

import (
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/k0sproject/k0s/pkg/apis/k0s/v1beta1"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"

	"sigs.k8s.io/yaml"

	"github.com/k0sproject/k0s/inttest/common"
	"github.com/stretchr/testify/suite"
)

type KubeRouterHairpinSuite struct {
	common.BootlooseSuite
	isIPv6Only bool
}

func (s *KubeRouterHairpinSuite) TestK0sGetsUp() {
	{
		clusterCfg := &v1beta1.ClusterConfig{
			Spec: &v1beta1.ClusterSpec{
				Network: func() *v1beta1.Network {
					network := v1beta1.DefaultNetwork()
					network.Provider = "kuberouter"
					network.KubeProxy = &v1beta1.KubeProxy{
						NodePortAddresses: []string{"127.0.0.0/24", "127.0.1.0/24"},
					}
					return network
				}(),
			},
		}
		if s.isIPv6Only {
			s.T().Log("Running in IPv6-only mode")
			clusterCfg.Spec.Network.PodCIDR = "fd00::/108"
			clusterCfg.Spec.Network.ServiceCIDR = "fd01::/108"
			clusterCfg.Spec.Network.KubeProxy.NodePortAddresses = []string{"::1/96"}
		}

		config, err := yaml.Marshal(clusterCfg)
		s.Require().NoError(err)
		s.WriteFileContent(s.ControllerNode(0), "/tmp/k0s.yaml", config)
	}

	s.Require().NoError(s.InitController(0, "--config=/tmp/k0s.yaml", "--disable-components=konnectivity-server,metrics-server", "--feature-gates=IPv6SingleStack=true"))

	if s.isIPv6Only {
		s.T().Log("Setting up IPv6 DNS for workers")
		common.ConfigureIPv6ResolvConf(&s.BootlooseSuite)
	}
	s.MakeDir(s.ControllerNode(0), "/var/lib/k0s/manifests/test")
	s.PutFile(s.ControllerNode(0), "/var/lib/k0s/manifests/test/pod.yaml", podManifest)
	s.PutFile(s.ControllerNode(0), "/var/lib/k0s/manifests/test/service.yaml", serviceManifest)
	s.Require().NoError(s.RunWorkers())

	kc, err := s.KubeClient("controller0", "")
	s.Require().NoError(err)

	err = s.WaitForNodeReady("worker0", kc)
	s.NoError(err)

	err = s.WaitForNodeReady("worker1", kc)
	s.NoError(err)

	s.T().Log("waiting to see kube-router pods ready")
	s.NoError(common.WaitForKubeRouterReady(s.Context(), kc), "kube-router did not start")

	s.T().Log("waiting to see hairpin pod ready")
	err = common.WaitForPod(s.Context(), kc, "hairpin-pod", metav1.NamespaceDefault)
	s.Require().NoError(err)

	s.Run("check hairpin mode", func() {
		// All done via SSH as it's much simpler :)
		// e.g. execing via client-go is super complex and would require too much wiring
		ssh, err := s.SSH(s.Context(), s.ControllerNode(0))
		s.Require().NoError(err)
		defer ssh.Disconnect()

		const curl = "k0s kc exec -n default hairpin-pod -c curl -- curl"
		for _, test := range []struct {
			dnsName string
			desc    string
		}{
			{
				"localhost",
				"pod can reach itself via loopback",
			},
			{
				"hairpin",
				"pod can reach itself via service name",
			},
		} {
			s.Run(test.desc, func() {
				err = wait.PollImmediate(5*time.Second, 2*time.Minute, func() (bool, error) {
					output, err := ssh.ExecWithOutput(s.Context(), fmt.Sprintf("%s --connect-timeout 5 -sS http://%s", curl, test.dnsName))
					if err != nil {
						s.T().Log(output)
						return false, nil
					}
					return s.Contains(output, "Thank you for using nginx."), nil
				})
				s.Require().NoError(err)
			})
		}
	})
}

func TestKubeRouterHairpinSuite(t *testing.T) {
	s := KubeRouterHairpinSuite{
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

const podManifest = `
apiVersion: v1
kind: Pod
metadata:
  name: hairpin-pod
  namespace: default
  labels:
    app.kubernetes.io/name: hairpin
spec:
  containers:
  - name: nginx
    image: docker.io/library/nginx:1.29.4-alpine
    ports:
    - containerPort: 80
  - name: curl
    image: docker.io/curlimages/curl:8.18.0
    command: ["/bin/sh", "-c"]
    args: ["tail -f /dev/null"]
`

const serviceManifest = `
apiVersion: v1
kind: Service
metadata:
  name: hairpin
  namespace: default
spec:
  selector:
    app.kubernetes.io/name: hairpin
  ports:
  - protocol: TCP
    port: 80
    targetPort: 80
`
