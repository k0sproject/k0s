/*
Copyright 2024 k0s authors

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

package openebs

import (
	"context"
	"testing"
	"time"

	"github.com/k0sproject/bootloose/pkg/config"
	"github.com/k0sproject/k0s/inttest/common"
	"github.com/k0sproject/k0s/pkg/kubernetes/watch"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/kubernetes"

	helmv1beta1 "github.com/k0sproject/k0s/pkg/apis/helm/v1beta1"
	helmclient "github.com/k0sproject/k0s/pkg/client/clientset/typed/helm/v1beta1"
	"github.com/stretchr/testify/suite"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type OpenEBSSuite struct {
	common.BootlooseSuite
}

func (s *OpenEBSSuite) TestK0sGetsUp() {
	ctx := s.Context()

	s.T().Log("Start k0s with both storage and helm extensions enabled")
	s.PutFile(s.ControllerNode(0), "/tmp/k0s.yaml", k0sConfigWithBoth)
	s.Require().NoError(s.InitController(0, "--config=/tmp/k0s.yaml", "--disable-components=konnectivity-server,metrics-server"))
	s.Require().NoError(s.RunWorkers())

	kc, err := s.KubeClient(s.ControllerNode(0), "")
	s.Require().NoError(err)

	s.Require().NoError(s.WaitForNodeReady(s.WorkerNode(0), kc))

	// When both storage and helm are enabled, there should be no action.
	// Unfortunately to check that, the best we can do is seeing that it's not
	// created after a grace period
	s.T().Log("Waiting 30 additional seconds of grace period to see if charts are created")
	s.sleep(ctx, 30*time.Second)

	s.T().Log("Checking that the chart isn't created")
	hc, err := s.HelmClient(s.ControllerNode(0), "")
	s.Require().NoError(err)
	_, err = hc.Charts("kube-system").Get(ctx, openEBSChart, metav1.GetOptions{})
	s.Require().True(errors.IsNotFound(err), "Chart was created when it shouldn't have been")

	// Verify Test as a storage extension
	s.T().Log("Retarting k0s with only storage extension enabled")
	s.Require().NoError(s.StopController(s.ControllerNode(0)))
	s.PutFile(s.ControllerNode(0), "/tmp/k0s.yaml", k0sConfigWithStorage)
	s.Require().NoError(s.StartController(s.ControllerNode(0)))
	s.Require().NoError(s.WaitForNodeReady(s.WorkerNode(0), kc))

	s.T().Log("Checking that the chart is created and ready")
	s.Require().NoError(s.waitForChartUpdated(ctx, "3.3.0"))
	s.waitForOpenEBSReady(ctx, kc)

	// Migrate to helm chart
	s.T().Log("Restarting k0s without applier-manager and without extension")
	s.Require().NoError(s.StopController(s.ControllerNode(0)))
	s.PutFile(s.ControllerNode(0), "/tmp/k0s.yaml", k0sConfigNoExtension)
	s.Require().NoError(s.InitController(0, "--config=/tmp/k0s.yaml", "--disable-components=konnectivity-server,metrics-server,applier-manager"))
	s.Require().NoError(s.WaitForNodeReady(s.WorkerNode(0), kc))

	s.T().Log("Removing Label and annotation")
	c, err := hc.Charts("kube-system").Get(ctx, openEBSChart, metav1.GetOptions{})
	s.Require().NoError(err, "Error getting OpenEBS chart after removing the storage extension")
	delete(c.Annotations, "k0s.k0sproject.io/stack-checksum")
	delete(c.Labels, "k0s.k0sproject.io/stack")
	_, err = hc.Charts("kube-system").Update(ctx, c, metav1.UpdateOptions{})
	s.Require().NoError(err, "Error removing stack applier information in OpenEBS chart")

	s.T().Log("Removing the manifest")
	ssh, err := s.SSH(s.Context(), s.ControllerNode(0))
	s.Require().NoError(err)
	defer ssh.Disconnect()
	s.Require().NoError(ssh.Exec(ctx, "rm -f /var/lib/k0s/manifests/helm/0_helm_extension_openebs.yaml", common.SSHStreams{}))

	s.T().Log("Upgrading to 3.9.0")
	c, err = hc.Charts("kube-system").Get(ctx, openEBSChart, metav1.GetOptions{})
	s.Require().NoError(err, "Error getting OpenEBS chart after removing the storage extension")
	c.Spec.Version = "3.9.0"
	_, err = hc.Charts("kube-system").Update(ctx, c, metav1.UpdateOptions{})
	s.Require().NoError(err, "Error upgrading OpenEBS chart")

	s.T().Log("Checking that the chart is upgrades to 3.9.0 and becomes ready")
	s.Require().NoError(s.waitForChartUpdated(ctx, "3.9.0"))
	s.waitForOpenEBSReady(ctx, kc)

	// Test that applier doesn't revert it back to 3.9.0
	s.T().Log("Restarting the controller with manifest applier")
	s.Require().NoError(s.InitController(0, "--config=/tmp/k0s.yaml", "--disable-components=konnectivity-server,metrics-server"))
	s.Require().NoError(s.WaitForNodeReady(s.WorkerNode(0), kc))

	s.T().Log("Waiting 30 additional seconds of grace period to see if charts is deleted")
	s.sleep(ctx, 30*time.Second)

	s.T().Log("Checking that the chart is still to 3.9.0 and ready")
	s.Require().NoError(s.waitForChartUpdated(ctx, "3.9.0"))
	s.waitForOpenEBSReady(ctx, kc)
}

func TestOpenEBSSuite(t *testing.T) {
	s := OpenEBSSuite{
		common.BootlooseSuite{
			ControllerCount: 1,
			WorkerCount:     1,
			ExtraVolumes: []config.Volume{{
				Type:        "bind",
				Source:      "/run/udev",
				Destination: "/run/udev",
				ReadOnly:    false,
			}},
		},
	}
	suite.Run(t, &s)
}

func (s *OpenEBSSuite) waitForChartUpdated(ctx context.Context, version string) error {
	hc, err := s.HelmClient(s.ControllerNode(0), "")
	s.Require().NoError(err)

	return watch.Charts(hc.Charts("kube-system")).
		WithObjectName(openEBSChart).
		WithErrorCallback(common.RetryWatchErrors(s.T().Logf)).
		Until(ctx, func(chart *helmv1beta1.Chart) (done bool, err error) {
			// We don't need to actually deploy helm in this test
			// we're just validation that the spec is correct
			return chart.Spec.Version == version &&
				chart.Status.AppVersion == version &&
				chart.Status.Version == version, nil
		})

}

func (s *OpenEBSSuite) waitForOpenEBSReady(ctx context.Context, kc *kubernetes.Clientset) {
	s.T().Log("Waiting for openEBS to be ready")
	s.Require().NoError(common.WaitForDeployment(ctx, kc, "openebs-localpv-provisioner", "openebs"))
	s.Require().NoError(common.WaitForDeployment(ctx, kc, "openebs-ndm-operator", "openebs"))
	s.Require().NoError(common.WaitForDaemonSet(ctx, kc, "openebs-ndm", "openebs"))
}

// HelmClient returns HelmV1beta1Client by loading the admin access config from given node
func (s *OpenEBSSuite) HelmClient(node string, k0sKubeconfigArgs ...string) (*helmclient.HelmV1beta1Client, error) {
	cfg, err := s.GetKubeConfig(node, k0sKubeconfigArgs...)
	if err != nil {
		return nil, err
	}
	return helmclient.NewForConfig(cfg)
}

func (s *OpenEBSSuite) sleep(ctx context.Context, d time.Duration) {
	select {
	case <-ctx.Done():
		s.Require().NoError(ctx.Err())
	case <-time.After(30 * time.Second):
	}
}

const k0sConfigWithBoth = `
spec:
  extensions:
    storage:
      type: openebs_local_storage
    helm:
      repositories:
      - name: openebs-internal
        url: https://openebs.github.io/charts
      charts:
      - name: openebs
        chartname: openebs-internal/openebs
        version: "3.9.0"
        namespace: openebs
        order: 1
        values: |
          localprovisioner:
            hostpathClass:
              enabled: true
              isDefaultClass: false
`

const k0sConfigWithStorage = `
spec:
  extensions:
    storage:
      type: openebs_local_storage
`

const k0sConfigNoExtension = `
spec:
  extensions: {}
`
const openEBSChart = "k0s-addon-chart-openebs"
