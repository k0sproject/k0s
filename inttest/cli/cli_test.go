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

package cli

import (
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/k0sproject/k0s/inttest/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type CliSuite struct {
	common.FootlooseSuite
}

func (s *CliSuite) TestK0sCliCommandNegative() {
	ssh, err := s.SSH(s.Context(), s.ControllerNode(0))
	s.Require().NoError(err)
	defer ssh.Disconnect()

	// k0s controller command should fail if non existent path to config is passed
	_, err = ssh.ExecWithOutput(s.Context(), "/usr/local/bin/k0s controller --config /some/fake/path")
	s.Require().Error(err)

	// k0s install command should fail if non existent path to config is passed
	_, err = ssh.ExecWithOutput(s.Context(), "/usr/local/bin/k0s install controller --config /some/fake/path")
	s.Require().Error(err)

	// k0s start should fail if service is not installed
	_, err = ssh.ExecWithOutput(s.Context(), "/usr/local/bin/k0s start")
	s.Require().Error(err)

	// k0s stop should fail if service is not installed
	_, err = ssh.ExecWithOutput(s.Context(), "/usr/local/bin/k0s stop")
	s.Require().Error(err)
}

func (s *CliSuite) TestK0sCliKubectlAndResetCommand() {
	ssh, err := s.SSH(s.Context(), s.ControllerNode(0))
	s.Require().NoError(err, "failed to SSH into controller")
	defer ssh.Disconnect()

	s.T().Run("sysinfoSmoketest", func(t *testing.T) {
		out, err := ssh.ExecWithOutput(s.Context(), fmt.Sprintf("%s sysinfo", s.K0sFullPath))
		assert.NoError(t, err, "k0s sysinfo has non-zero exit code")
		t.Logf(out)
		assert.Regexp(t, "^Machine ID: ", out)
		assert.Regexp(t, "\nOperating system: Linux \\(pass\\)\n", out)
		assert.Regexp(t, "\n  Linux kernel release: ", out)
		assert.Regexp(t, "\n  CONFIG_CGROUPS: ", out)
		assert.Regexp(t, "\n  Control Groups: ", out)
		assert.Regexp(t, "\n    cgroup controller \"[a-z]+\": ", out)
	})

	s.T().Run("k0sInstall", func(t *testing.T) {
		// Install with some arbitrary kubelet flags so we see those get properly passed to the kubelet
		out, err := ssh.ExecWithOutput(s.Context(), "/usr/local/bin/k0s install controller --enable-worker --disable-components konnectivity-server,metrics-server --kubelet-extra-args='--event-qps=7 --enable-load-reader=true'")
		assert.NoError(t, err)
		assert.Equal(t, "", out)
	})

	s.T().Run("k0sStart", func(t *testing.T) {
		assert := assert.New(t)
		require := require.New(t)

		_, err = ssh.ExecWithOutput(s.Context(), "/usr/local/bin/k0s start")
		require.NoError(err)

		require.NoError(s.WaitForKubeAPI(s.ControllerNode(0)))

		output, err := ssh.ExecWithOutput(s.Context(), "/usr/local/bin/k0s kubectl get namespaces -o json 2>/dev/null")
		require.NoError(err)

		namespaces := &K8sNamespaces{}
		assert.NoError(json.Unmarshal([]byte(output), namespaces))

		assert.Len(namespaces.Items, 4)
		assert.Equal("default", namespaces.Items[0].Metadata.Name)
		assert.Equal("kube-node-lease", namespaces.Items[1].Metadata.Name)
		assert.Equal("kube-public", namespaces.Items[2].Metadata.Name)
		assert.Equal("kube-system", namespaces.Items[3].Metadata.Name)

		kc, err := s.KubeClient(s.ControllerNode(0))
		require.NoError(err)

		err = s.WaitForNodeReady(s.ControllerNode(0), kc)
		assert.NoError(err)

		s.AssertSomeKubeSystemPods(kc)

		// Wait till we see all pods running, otherwise we get into weird timing issues and high probability of leaked containerd shim processes
		require.NoError(common.WaitForDaemonSet(s.Context(), kc, "kube-proxy"))
		require.NoError(common.WaitForKubeRouterReady(s.Context(), kc))
		require.NoError(common.WaitForDeployment(s.Context(), kc, "coredns", "kube-system"))

		// Check that the kubelet extra flags are properly set
		kubeletCmdLine, err := s.GetKubeletCMDLine(s.ControllerNode(0))
		s.Require().NoError(err)
		s.Require().Contains(kubeletCmdLine, "--event-qps=7")
		s.Require().Contains(kubeletCmdLine, "--enable-load-reader=true")
	})

	s.T().Log("waiting for k0s to terminate")
	_, err = ssh.ExecWithOutput(s.Context(), "/usr/local/bin/k0s stop")
	s.Require().NoError(err)
	_, err = ssh.ExecWithOutput(s.Context(), "while pidof k0s containerd kubelet; do sleep 0.1s; done")
	s.Require().NoError(err)

	s.T().Run("k0sReset", func(t *testing.T) {
		assert := assert.New(t)
		resetOutput, err := ssh.ExecWithOutput(s.Context(), "/usr/local/bin/k0s reset --debug")
		s.T().Logf("Reset executed with output:\n%s", resetOutput)

		// k0s reset will always exit with an error on footloose, since it's unable to remove /var/lib/k0s
		// that is an expected behaviour. therefore, we're only checking if the contents of /var/lib/k0s is empty
		assert.Error(err)

		fileCount, err := ssh.ExecWithOutput(s.Context(), "find /var/lib/k0s -type f | wc -l")
		assert.NoError(err)
		assert.Equal("0", fileCount, "expected to see 0 files under /var/lib/k0s")

		newPodCount, err := ssh.ExecWithOutput(s.Context(), "ps aux | grep '[c]ontainerd-shim-runc-v2' | wc -l")
		assert.NoError(err)
		assert.Equal("0", newPodCount, "expected to see 0 pods after reset command")
	})
}

func TestCliCommandSuite(t *testing.T) {
	s := CliSuite{
		common.FootlooseSuite{
			ControllerCount: 1,
		},
	}
	suite.Run(t, &s)
}

type K8sNamespaces struct {
	APIVersion string `json:"apiVersion"`
	Items      []struct {
		APIVersion string `json:"apiVersion"`
		Kind       string `json:"kind"`
		Metadata   struct {
			CreationTimestamp time.Time `json:"creationTimestamp"`
			Labels            struct {
				KubernetesIoMetadataName string `json:"kubernetes.io/metadata.name"`
			} `json:"labels"`
			ManagedFields []struct {
				APIVersion string `json:"apiVersion"`
				FieldsType string `json:"fieldsType"`
				FieldsV1   struct {
					FMetadata struct {
						FLabels struct {
							FKubernetesIoMetadataName struct {
							} `json:"f:kubernetes.io/metadata.name"`
						} `json:"f:labels"`
					} `json:"f:metadata"`
				} `json:"fieldsV1"`
				Manager   string    `json:"manager"`
				Operation string    `json:"operation"`
				Time      time.Time `json:"time"`
			} `json:"managedFields"`
			Name            string `json:"name"`
			ResourceVersion string `json:"resourceVersion"`
			UID             string `json:"uid"`
		} `json:"metadata"`
		Spec struct {
			Finalizers []string `json:"finalizers"`
		} `json:"spec"`
		Status struct {
			Phase string `json:"phase"`
		} `json:"status"`
	} `json:"items"`
	Kind     string `json:"kind"`
	Metadata struct {
		ResourceVersion string `json:"resourceVersion"`
		SelfLink        string `json:"selfLink"`
	} `json:"metadata"`
}
