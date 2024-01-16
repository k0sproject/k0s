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

package airgap

import (
	"fmt"
	"strings"
	"testing"

	"github.com/k0sproject/k0s/inttest/common"
	"github.com/k0sproject/k0s/pkg/airgap"
	"github.com/k0sproject/k0s/pkg/apis/k0s/v1beta1"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"

	"github.com/stretchr/testify/suite"
)

const k0sConfig = `
spec:
  images:
    default_pull_policy: Never
`

type AirgapSuite struct {
	common.BootlooseSuite
}

func (s *AirgapSuite) TestK0sGetsUp() {
	ctx := s.Context()
	err := (&common.Airgap{
		SSH:  s.SSH,
		Logf: s.T().Logf,
	}).LockdownMachines(ctx,
		s.ControllerNode(0), s.WorkerNode(0),
	)
	s.Require().NoError(err)

	s.PutFile(s.ControllerNode(0), "/tmp/k0s.yaml", k0sConfig)
	s.NoError(s.InitController(0, "--config=/tmp/k0s.yaml"))
	s.NoError(s.RunWorkers(`--labels="k0sproject.io/foo=bar"`, `--kubelet-extra-args="--address=0.0.0.0 --event-burst=10 --image-gc-high-threshold=100"`))

	kc, err := s.KubeClient(s.ControllerNode(0))
	s.Require().NoError(err)

	err = s.WaitForNodeReady(s.WorkerNode(0), kc)
	s.NoError(err)

	if labels, err := s.GetNodeLabels(s.WorkerNode(0), kc); s.NoError(err) {
		s.Equal("bar", labels["k0sproject.io/foo"])
	}

	s.Require().NoError(common.WaitForKubeRouterReady(ctx, kc), "While waiting for kube-router to become ready")
	s.Require().NoError(common.WaitForCoreDNSReady(ctx, kc), "While waiting for CoreDNS to become ready")
	s.Require().NoError(common.WaitForPodLogs(ctx, kc, "kube-system"), "While waiting for some pod logs")

	// At that moment we can assume that all pods have at least started
	// We're interested only in image pull events
	events, err := kc.CoreV1().Events("").List(ctx, metav1.ListOptions{
		FieldSelector: fields.AndSelectors(
			fields.OneTermEqualSelector("involvedObject.kind", "Pod"),
			fields.OneTermEqualSelector("reason", "Pulled"),
		).String(),
	})
	s.Require().NoError(err)

	for _, event := range events.Items {
		if !strings.HasSuffix(event.Message, "already present on machine") {
			s.Fail("Unexpected Pulled event", event.Message)
		} else {
			s.T().Log("Observed Pulled event:", event.Message)
		}
	}

	// Check that all the images have io.cri-containerd.pinned=pinned label
	ssh, err := s.SSH(ctx, s.WorkerNode(0))
	s.Require().NoError(err)
	defer ssh.Disconnect()
	for _, i := range airgap.GetImageURIs(v1beta1.DefaultClusterSpec(), true) {
		output, err := ssh.ExecWithOutput(ctx, fmt.Sprintf(`k0s ctr i ls "name==%s"`, i))
		s.Require().NoError(err)
		s.Require().Contains(output, "io.cri-containerd.pinned=pinned", "expected %s image to have io.cri-containerd.pinned=pinned label", i)
	}
}

func TestAirgapSuite(t *testing.T) {
	s := AirgapSuite{
		common.BootlooseSuite{
			ControllerCount: 1,
			WorkerCount:     1,

			AirgapImageBundleMountPoints: []string{"/var/lib/k0s/images/bundle.tar"},
		},
	}
	suite.Run(t, &s)
}
