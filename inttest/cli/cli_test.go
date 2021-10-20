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
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/k0sproject/k0s/inttest/common"
	"github.com/stretchr/testify/suite"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type CliSuite struct {
	common.FootlooseSuite
}

func (s *CliSuite) TestK0sCliCommandNegative() {
	ssh, err := s.SSH(s.ControllerNode(0))
	s.Require().NoError(err)
	defer ssh.Disconnect()

	// k0s controller command should fail if non existent path to config is passed
	_, err = ssh.ExecWithOutput("k0s controller --config /some/fake/path")
	s.Require().Error(err)

	// k0s install command should fail if non existent path to config is passed
	_, err = ssh.ExecWithOutput("k0s install controller --config /some/fake/path")
	s.Require().Error(err)

	// k0s start should fail if service is not installed
	_, err = ssh.ExecWithOutput("k0s start")
	s.Require().Error(err)

	// k0s stop should fail if service is not installed
	_, err = ssh.ExecWithOutput("k0s stop")
	s.Require().Error(err)
}

func (s *CliSuite) TestK0sCliKubectlAndResetCommand() {
	ssh, err := s.SSH(s.ControllerNode(0))
	s.Require().NoError(err)
	defer ssh.Disconnect()

	s.T().Log("running k0s install command")
	_, err = ssh.ExecWithOutput("k0s install controller --enable-worker --disable-components konnectivity-server,metrics-server")
	s.Require().NoError(err)

	_, err = ssh.ExecWithOutput("k0s start")
	s.Require().NoError(err)

	err = s.WaitForKubeAPI(s.ControllerNode(0))
	s.Require().NoError(err)

	output, err := ssh.ExecWithOutput("k0s kubectl get namespaces -o json")
	s.Require().NoError(err)

	namespaces := &K8sNamespaces{}

	err = json.Unmarshal([]byte(output), namespaces)
	s.NoError(err)

	s.Len(namespaces.Items, 4)
	s.Equal("default", namespaces.Items[0].Metadata.Name)
	s.Equal("kube-node-lease", namespaces.Items[1].Metadata.Name)
	s.Equal("kube-public", namespaces.Items[2].Metadata.Name)
	s.Equal("kube-system", namespaces.Items[3].Metadata.Name)

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

	// Wait till we see all pods running, otherwise we get into weird timing issues and high probability of leaked containerd shim processes
	s.Require().NoError(common.WaitForDaemonSet(kc, "kube-proxy"))
	s.Require().NoError(common.WaitForKubeRouterReady(kc))
	s.Require().NoError(common.WaitForDeployment(kc, "coredns"))

	// Stop and actually wait till k0s dies
	_, err = ssh.ExecWithOutput("k0s stop && while pidof k0s containerd kubelet; do sleep 0.1s; done")
	s.Require().NoError(err)

	s.T().Log("running k0s reset command")
	// k0s reset will always exit with an error on footloose, since it's unable to remove /var/lib/k0s
	// that is an expected behaviour. therefore, we're only checking if the contents of /var/lib/k0s is empty
	resetOutput, _ := ssh.ExecWithOutput("k0s reset --debug")

	s.T().Logf("Reset executed with output:\n%s", resetOutput)

	fileCount, _ := ssh.ExecWithOutput("find /var/lib/k0s -type f | wc -l")
	s.Equal("0", fileCount, "expected to see 0 files under /var/lib/k0s")

	newPodCount, _ := ssh.ExecWithOutput("ps aux | grep '[c]ontainerd-shim-runc-v2' | wc -l")
	s.Equal("0", newPodCount, "expected to see 0 pods after reset command")
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
