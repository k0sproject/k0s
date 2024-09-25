// Copyright 2023 k0s authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package updater

import (
	"fmt"
	"testing"
	"time"

	"github.com/k0sproject/k0s/inttest/common"
	aptest "github.com/k0sproject/k0s/inttest/common/autopilot"
	k0sclientset "github.com/k0sproject/k0s/pkg/client/clientset"

	apconst "github.com/k0sproject/k0s/pkg/autopilot/constant"
	appc "github.com/k0sproject/k0s/pkg/autopilot/controller/plans/core"
	"github.com/k0sproject/k0s/pkg/kubernetes/watch"
	"github.com/stretchr/testify/suite"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

const (
	ManifestTestDirPerms = "775"
)

type plansSingleControllerSuite struct {
	common.BootlooseSuite
}

var envTemplate = `
export K0S_UPDATE_SERVER={{.Address}}
export K0S_UPDATE_PERIOD=1m
export K0S_UPDATE_CHECK_INTERVAL=1m
`

// SetupTest prepares the controller and filesystem, getting it into a consistent
// state which we can run tests against.
func (s *plansSingleControllerSuite) SetupTest() {
	ctx := s.Context()
	s.Require().NoError(s.WaitForSSH(s.ControllerNode(0), 2*time.Minute, 1*time.Second))

	// Dump some env vars for testing in /etc/conf.d/k0scontroller
	vars := struct {
		Address string
	}{
		Address: fmt.Sprintf("http://%s", s.GetUpdateServerIPAddress()),
	}
	s.PutFileTemplate(s.ControllerNode(0), "/etc/conf.d/k0scontroller", envTemplate, vars)

	s.Require().NoError(s.InitController(0, "--disable-components=metrics-server"))
	s.Require().NoError(s.WaitJoinAPI(s.ControllerNode(0)))

	kc, err := s.KubeClient(s.ControllerNode(0))
	s.Require().NoError(err)

	client, err := s.ExtensionsClient(s.ControllerNode(0))
	s.Require().NoError(err)

	s.Require().NoError(aptest.WaitForCRDByName(ctx, client, "plans"))
	s.Require().NoError(aptest.WaitForCRDByName(ctx, client, "controlnodes"))
	s.Require().NoError(aptest.WaitForCRDByName(ctx, client, "updateconfigs"))
	// Wait that we see an event for update before proceeding to actual update testing
	err = watch.Events(kc.CoreV1().Events("")).
		Until(s.Context(), func(e *corev1.Event) (done bool, err error) {
			return e.Type == "Normal" &&
				e.Source.Component == "k0s" &&
				e.Reason == "NewVersionAvailable", nil
		})
	s.Require().NoError(err)
	// Get the first line of access logs to verify that the update headers are present
	ssh, err := s.SSH(s.Context(), "updateserver0")
	s.Require().NoError(err)
	defer ssh.Disconnect()
	logs, err := ssh.ExecWithOutput(s.Context(), "head -1 /var/log/nginx/access.log")
	s.Require().NoError(err)
	s.verifyUpdateHeaders(kc, logs)
}

func (s *plansSingleControllerSuite) verifyUpdateHeaders(kc kubernetes.Interface, logLine string) {
	// Verify that the update headers are present in the update server logs
	s.Require().Contains(logLine, `K0S_StorageType="etcd"`)
	s.Require().Contains(logLine, "K0S_ControlPlaneNodesCount=1")
	s.Require().Contains(logLine, fmt.Sprintf(`K0S_ClusterID="%s"`, s.getClusterID(kc)))
	s.Require().Contains(logLine, `K0S_CNIProvider="kuberouter"`)
}

func (s *plansSingleControllerSuite) getClusterID(kc kubernetes.Interface) string {
	ns, err := kc.CoreV1().Namespaces().Get(s.Context(), "kube-system", metav1.GetOptions{})
	s.Require().NoError(err)
	return fmt.Sprintf("%s:%s", ns.Name, ns.UID)
}

// TestApply applies a well-formed `plan` yaml, and asserts that all of the correct values
// across different objects are correct.
func (s *plansSingleControllerSuite) TestApply() {
	ctx := s.Context()

	restConfig, err := s.GetKubeConfig(s.ControllerNode(0))
	s.Require().NoError(err)

	updaterConfig := `
apiVersion: autopilot.k0sproject.io/v1beta2
kind: UpdateConfig
metadata:
  name: autopilot
spec:
  channel: latest
  updateServer: http://` + s.GetUpdateServerIPAddress() + `
  upgradeStrategy:
    type: periodic
    periodic:
      days: [Monday, Tuesday, Wednesday, Thursday, Friday, Saturday, Sunday]
      startTime: 00:00
      length: 24h
  planSpec:
    commands:
    - k0supdate:
        forceupdate: true
        targets:
          controllers:
            discovery:
              selector: {}
          workers:
            discovery:
              selector: {}
`

	_, err = common.Create(ctx, restConfig, []byte(updaterConfig))
	s.Require().NoError(err)
	s.T().Logf("Plan created")

	// The plan has enough information to perform a successful update of k0s, so wait for it.
	client, err := k0sclientset.NewForConfig(restConfig)
	s.Require().NoError(err)
	_, err = aptest.WaitForPlanState(ctx, client, apconst.AutopilotName, appc.PlanCompleted)
	s.Require().NoError(err)

	kc, err := s.KubeClient(s.ControllerNode(0))
	s.Require().NoError(err)

	// Verify that the update headers are present in the update server logs,
	// ignoring the "grab" user-agent as that's the actual k0s bin download which we don't care
	ssh, err := s.SSH(s.Context(), "updateserver0")
	s.Require().NoError(err)
	defer ssh.Disconnect()
	logs, err := ssh.ExecWithOutput(s.Context(), "grep -v grab /var/log/nginx/access.log | tail -1")
	s.Require().NoError(err)

	s.verifyUpdateHeaders(kc, logs)

}

// TestPlansSingleControllerSuite sets up a suite using a single controller, running various
// autopilot upgrade scenarios against it.
func TestPlansSingleControllerSuite(t *testing.T) {
	suite.Run(t, &plansSingleControllerSuite{
		common.BootlooseSuite{
			ControllerCount:  1,
			WorkerCount:      0,
			WithUpdateServer: true,
			LaunchMode:       common.LaunchModeOpenRC,
		},
	})
}
