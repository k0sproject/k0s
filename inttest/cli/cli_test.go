// SPDX-FileCopyrightText: 2021 k0s authors
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"encoding/json"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/k0sproject/k0s/inttest/common"
	"github.com/stretchr/testify/suite"
)

type CliSuite struct {
	common.BootlooseSuite
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

	s.Run("sysinfoSmoketest", func() {
		out, err := ssh.ExecWithOutput(s.Context(), s.K0sFullPath+" sysinfo")
		s.NoError(err, "k0s sysinfo has non-zero exit code")
		s.T().Logf("%s", out)
		s.Regexp("\nOperating system: Linux \\(pass\\)\n", out)
		s.Regexp("\n  Linux kernel release: ", out)
		s.Regexp("\n  CONFIG_CGROUPS: ", out)
		s.Regexp("\n  Control Groups: ", out)
		s.Regexp("\n    cgroup controller \"[a-z]+\": ", out)
	})

	s.Run("k0sInstall", func() {
		// Install with some arbitrary kubelet flags so we see those get properly passed to the kubelet
		out, err := ssh.ExecWithOutput(s.Context(), "/usr/local/bin/k0s install controller --enable-worker --disable-components konnectivity-server,metrics-server --kubelet-extra-args='--housekeeping-interval=10s --log-flush-frequency=5s'")
		s.NoError(err)
		s.Empty(out)
	})

	s.Run("k0sStart", func() {
		assert := s.Assertions
		require := s.Require()

		_, err = ssh.ExecWithOutput(s.Context(), "/usr/local/bin/k0s start")
		require.NoError(err)

		require.NoError(s.WaitForKubeAPI(s.ControllerNode(0)))

		output, err := ssh.ExecWithOutput(s.Context(), "/usr/local/bin/k0s kubectl get namespaces -o json 2>/dev/null")
		require.NoError(err)

		namespaces := &K8sNamespaces{}
		assert.NoError(json.Unmarshal([]byte(output), namespaces))

		assert.Len(namespaces.Items, 4)
		assert.Equal(metav1.NamespaceDefault, namespaces.Items[0].Metadata.Name)
		assert.Equal(corev1.NamespaceNodeLease, namespaces.Items[1].Metadata.Name)
		assert.Equal(metav1.NamespacePublic, namespaces.Items[2].Metadata.Name)
		assert.Equal(metav1.NamespaceSystem, namespaces.Items[3].Metadata.Name)

		kc, err := s.KubeClient(s.ControllerNode(0))
		require.NoError(err)

		err = s.WaitForNodeReady(s.ControllerNode(0), kc)
		assert.NoError(err)

		s.AssertSomeKubeSystemPods(kc)

		// Wait till we see all pods running, otherwise we get into weird timing issues and high probability of leaked containerd shim processes
		require.NoError(common.WaitForDaemonSet(s.Context(), kc, "kube-proxy", metav1.NamespaceSystem))
		require.NoError(common.WaitForKubeRouterReady(s.Context(), kc))
		require.NoError(common.WaitForDeployment(s.Context(), kc, "coredns", metav1.NamespaceSystem))

		// Check that the kubelet extra flags are properly set
		kubeletCmdLine, err := s.GetKubeletCMDLine(s.ControllerNode(0))
		require.NoError(err)
		assert.Contains(kubeletCmdLine, "--housekeeping-interval=10s")
		assert.Contains(kubeletCmdLine, "--log-flush-frequency=5s")
	})

	s.T().Log("waiting for k0s to terminate")
	_, err = ssh.ExecWithOutput(s.Context(), "/usr/local/bin/k0s stop")
	s.Require().NoError(err)
	_, err = ssh.ExecWithOutput(s.Context(), "while pidof k0s containerd kubelet; do sleep 0.1s; done")
	s.Require().NoError(err)

	s.Run("k0sReset", func() {
		assert := s.Assertions
		resetOutput, err := ssh.ExecWithOutput(s.Context(), "/usr/local/bin/k0s reset --debug")
		s.T().Logf("Reset executed with output:\n%s", resetOutput)

		assert.NoError(err)

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
		common.BootlooseSuite{
			ControllerCount: 1,
			// The tests start and stop k0s manually. Setting the launch mode to
			// OpenRC here anyways, so that the log collection will pick up the
			// right paths.
			LaunchMode: common.LaunchModeOpenRC,
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
