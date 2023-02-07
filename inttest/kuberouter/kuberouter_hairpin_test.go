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

package kuberouter

import (
	"fmt"
	"testing"

	"github.com/k0sproject/k0s/inttest/common"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type KubeRouterHairpinSuite struct {
	common.FootlooseSuite
}

func (s *KubeRouterHairpinSuite) TestK0sGetsUp() {
	s.PutFile(s.ControllerNode(0), "/tmp/k0s.yaml", k0sConfigWithHairpinning)
	s.Require().NoError(s.InitController(0, "--config=/tmp/k0s.yaml", "--disable-components=konnectivity-server,metrics-server"))
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
	err = common.WaitForPod(s.Context(), kc, "hairpin-pod", "default")
	s.Require().NoError(err)

	s.T().Run("check hairpin mode", func(t *testing.T) {
		// All done via SSH as it's much simpler :)
		// e.g. execing via client-go is super complex and would require too much wiring
		ssh, err := s.SSH(s.ControllerNode(0))
		require.NoError(t, err)
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
			t.Run(test.desc, func(t *testing.T) {
				output, err := ssh.ExecWithOutput(s.Context(), fmt.Sprintf("%s --connect-timeout 5 -sS http://%s", curl, test.dnsName))
				if !assert.NoError(t, err) || !assert.Contains(t, output, "Thank you for using nginx.") {
					t.Log(output)
				}
			})
		}
	})
}

func TestKubeRouterHairpinSuite(t *testing.T) {
	s := KubeRouterHairpinSuite{
		common.FootlooseSuite{
			ControllerCount: 1,
			WorkerCount:     2,
		},
	}
	suite.Run(t, &s)
}

const k0sConfigWithHairpinning = `
spec:
  network:
    provider: kuberouter
`

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
    image: docker.io/library/nginx:1.23.1-alpine
    ports:
    - containerPort: 80
  - name: curl
    image: docker.io/curlimages/curl:7.84.0
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
